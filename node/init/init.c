// devlayer node init. One static binary, two stages.
//
// Stage 1 runs as /init in the initramfs. The kernel already brought the NIC
// up (ip=dhcp). Stage 1 mounts the EROFS sysroot read-only, puts a tmpfs
// overlay on top, and switches root. It then runs /sbin/init from the sysroot.
//
// Stage 2 is /sbin/init in the sysroot, PID 1. It mounts cgroup2 and runs one
// payload. A node runs /etc/node/hook when it exists. A test (node_test=1 on
// the kernel command line) runs /test/entry from the first virtio disk, sends
// stdout to hvc0 and stderr to hvc1, and exits QEMU with the payload status.
//
// The kernel passes unknown name=value boot options to init as environment
// variables. Init reads them with getenv. It parses nothing.
#define _GNU_SOURCE
#include <errno.h>
#include <fcntl.h>
#include <linux/loop.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/io.h>
#include <sys/ioctl.h>
#include <sys/mount.h>
#include <sys/reboot.h>
#include <sys/stat.h>
#include <sys/wait.h>
#include <unistd.h>

#define SYSROOT_IMAGE "/sysroot.erofs"
#define DEBUG_EXIT_PORT 0xf4

static void say(const char *fmt, const char *arg) {
    dprintf(2, "node-init: ");
    dprintf(2, fmt, arg);
    dprintf(2, "\n");
}

static void die(const char *what) {
    dprintf(2, "node-init: %s: %s\n", what, strerror(errno));
    sync();
    reboot(RB_AUTOBOOT);
    for (;;) pause();
}

static void must_mount(const char *src, const char *dst, const char *type,
                       unsigned long flags, const char *data) {
    mkdir(dst, 0755);
    if (mount(src, dst, type, flags, data) != 0) die(dst);
}

// Copies a whole file to fd. Used for /proc/net/pnp: it is already in
// resolv.conf form, so the bytes go out unchanged.
static void copy_file(const char *path, int out) {
    char buf[4096];
    int fd = open(path, O_RDONLY | O_CLOEXEC);
    if (fd < 0) return;
    ssize_t n;
    while ((n = read(fd, buf, sizeof buf)) > 0) write(out, buf, (size_t)n);
    close(fd);
}

static void attach_loop(const char *image, char *dev, size_t devlen) {
    int ctl = open("/dev/loop-control", O_RDWR | O_CLOEXEC);
    if (ctl < 0) die("/dev/loop-control");
    int nr = ioctl(ctl, LOOP_CTL_GET_FREE);
    if (nr < 0) die("LOOP_CTL_GET_FREE");
    close(ctl);
    snprintf(dev, devlen, "/dev/loop%d", nr);
    int lfd = open(dev, O_RDONLY | O_CLOEXEC);
    if (lfd < 0) die(dev);
    int ifd = open(image, O_RDONLY | O_CLOEXEC);
    if (ifd < 0) die(image);
    struct loop_config cfg;
    memset(&cfg, 0, sizeof cfg);
    cfg.fd = (unsigned)ifd;
    cfg.info.lo_flags = LO_FLAGS_READ_ONLY;
    if (ioctl(lfd, LOOP_CONFIGURE, &cfg) != 0) die("LOOP_CONFIGURE");
    close(ifd);
    close(lfd);
}

static int stage1(void) {
    must_mount("devtmpfs", "/dev", "devtmpfs", MS_NOSUID, "mode=0755");
    must_mount("proc", "/proc", "proc", MS_NOSUID | MS_NODEV | MS_NOEXEC, NULL);
    must_mount("sysfs", "/sys", "sysfs", MS_NOSUID | MS_NODEV | MS_NOEXEC, NULL);

    say("%s", "stage 1: network from the kernel (ip=dhcp):");
    copy_file("/proc/net/pnp", 2);

    char loopdev[32];
    attach_loop(SYSROOT_IMAGE, loopdev, sizeof loopdev);
    must_mount(loopdev, "/lower", "erofs", MS_RDONLY, NULL);
    say("sysroot %s mounted read-only", loopdev);

    // All writes land in RAM. /etc, /var, /run, /tmp, and home are writable.
    must_mount("tmpfs", "/rw", "tmpfs", 0, "mode=0755");
    mkdir("/rw/upper", 0755);
    mkdir("/rw/work", 0755);
    must_mount("overlay", "/newroot", "overlay", 0,
               "lowerdir=/lower,upperdir=/rw/upper,workdir=/rw/work");

    const char *moves[] = {"/dev", "/proc", "/sys"};
    for (size_t i = 0; i < sizeof moves / sizeof moves[0]; i++) {
        char dst[64];
        snprintf(dst, sizeof dst, "/newroot%s", moves[i]);
        mkdir(dst, 0755);
        if (mount(moves[i], dst, NULL, MS_MOVE, NULL) != 0) die(dst);
    }

    mkdir("/newroot/etc", 0755);
    int rc = open("/newroot/etc/resolv.conf", O_WRONLY | O_CREAT | O_TRUNC | O_CLOEXEC, 0644);
    if (rc >= 0) {
        copy_file("/newroot/proc/net/pnp", rc);
        close(rc);
    }

    // switch_root: free the initramfs, move the new root over /, chroot.
    // The loop device holds the image open, so its RAM stays until reboot.
    unlink(SYSROOT_IMAGE);
    unlink("/init");
    if (chdir("/newroot") != 0) die("chdir /newroot");
    if (mount(".", "/", NULL, MS_MOVE, NULL) != 0) die("move root");
    if (chroot(".") != 0) die("chroot");
    if (chdir("/") != 0) die("chdir /");

    char *argv[] = {"/sbin/init", NULL};
    execv(argv[0], argv);
    die("exec /sbin/init");
    return 1;
}

static void debug_exit(int status) {
    // QEMU isa-debug-exit: the process exits with (value << 1) | 1.
    if (ioperm(DEBUG_EXIT_PORT, 4, 1) == 0) outl((unsigned)status, DEBUG_EXIT_PORT);
}

static int open_console(const char *path, int fallback) {
    int fd = open(path, O_WRONLY | O_NOCTTY | O_CLOEXEC);
    return fd >= 0 ? fd : dup(fallback);
}

static int run_test(void) {
    must_mount("/dev/vda", "/test", "erofs", MS_RDONLY, NULL);
    mkdir("/tmp", 01777);
    must_mount("tmpfs", "/tmp", "tmpfs", MS_NOSUID | MS_NODEV, "mode=1777");
    int out = open_console("/dev/hvc0", 1);
    int err = open_console("/dev/hvc1", 2);

    pid_t pid = fork();
    if (pid < 0) die("fork");
    if (pid == 0) {
        dup2(out, 1);
        dup2(err, 2);
        if (chdir("/test/runfiles/_main") != 0) chdir("/test");
        setenv("RUNFILES_DIR", "/test/runfiles", 1);
        setenv("TEST_SRCDIR", "/test/runfiles", 1);
        setenv("TEST_WORKSPACE", "_main", 1);
        setenv("TEST_TMPDIR", "/tmp", 1);
        setenv("HOME", "/tmp", 1);
        setenv("PATH", "/bin:/usr/bin", 1);
        char *argv[] = {"/test/entry", NULL};
        execv(argv[0], argv);
        dprintf(2, "node-init: exec /test/entry: %s\n", strerror(errno));
        _exit(127);
    }
    int st = 0;
    while (waitpid(pid, &st, 0) < 0 && errno == EINTR) {}
    int code = WIFEXITED(st) ? WEXITSTATUS(st) : 128 + WTERMSIG(st);
    fsync(out);
    fsync(err);
    dprintf(2, "node-init: test exit %d\n", code);
    sync();
    debug_exit(code);
    reboot(RB_POWER_OFF);
    return code;
}

static int stage2(void) {
    must_mount("cgroup2", "/sys/fs/cgroup", "cgroup2", MS_NOSUID | MS_NODEV | MS_NOEXEC, NULL);
    must_mount("tmpfs", "/run", "tmpfs", MS_NOSUID | MS_NODEV, "mode=0755");
    say("%s", "stage 2: sysroot up, cgroup2 mounted");

    const char *t = getenv("node_test");
    if (t && strcmp(t, "1") == 0) return run_test();

    if (access("/etc/node/hook", X_OK) == 0) {
        pid_t pid = fork();
        if (pid == 0) {
            char *argv[] = {"/etc/node/hook", NULL};
            execv(argv[0], argv);
            _exit(127);
        }
    } else {
        say("%s", "no /etc/node/hook; idle");
    }
    say("%s", "online");
    for (;;) {
        if (wait(NULL) < 0 && errno == ECHILD) pause();
    }
}

int main(int argc, char **argv) {
    (void)argc;
    if (getpid() != 1) {
        dprintf(2, "node-init: not PID 1\n");
        return 1;
    }
    if (strcmp(argv[0], "/init") == 0) return stage1();
    return stage2();
}
