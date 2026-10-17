// devlayer node init. One static binary, three names.
//
// Stage 1 runs as /init in the initramfs. The kernel already brought the NIC
// up (ip=dhcp). Stage 1 mounts the runtime layer (/runtime.erofs, an EROFS
// image) read-only, puts a tmpfs overlay on top, and switches root. It then
// runs /sbin/init from the runtime layer.
//
// Stage 2 is /sbin/init in the runtime layer, PID 1. It mounts cgroup2,
// brings up lo, mounts extra layers, starts dropbear, and runs one payload.
// A node runs /etc/node/hook when it exists. A test (node_test=1 on the
// kernel command line) runs /test/entry from the virtio disk with serial
// "test", sends stdout to hvc0 and stderr to hvc1, and exits QEMU with the
// payload status.
//
// Layers. A layer is a read-only EROFS image named by the sha256 of its
// bytes. It mounts at /layers/.by-hash/<sha256>, and /layers/<name> is a
// symlink to that mount. A new version swaps the symlink with rename(2).
// At boot, a virtio disk with serial "layer-<name>" is mounted when the
// command line has node_layer_<name>=<sha256>. After boot,
//     node-layer <name> <sha256> <image-or-device>
// does the same for a file, for example one devlayer push sent over SSH.
// The hash is checked with sha256sum before the mount.
//
// The kernel passes unknown name=value boot options to init as environment
// variables. Init reads them with getenv.
#define _GNU_SOURCE
#include <dirent.h>
#include <errno.h>
#include <fcntl.h>
#include <linux/loop.h>
#include <net/if.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/io.h>
#include <sys/ioctl.h>
#include <sys/mount.h>
#include <sys/reboot.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/wait.h>
#include <unistd.h>

#define RUNTIME_IMAGE "/runtime.erofs"
#define DEBUG_EXIT_PORT 0xf4

static void say(const char *fmt, const char *arg) {
    dprintf(2, "node-init: ");
    dprintf(2, fmt, arg);
    dprintf(2, "\n");
}

static void die(const char *what) {
    dprintf(2, "node-init: %s: %s\n", what, strerror(errno));
    if (getpid() != 1) exit(1);
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
    attach_loop(RUNTIME_IMAGE, loopdev, sizeof loopdev);
    must_mount(loopdev, "/lower", "erofs", MS_RDONLY, NULL);
    say("runtime layer %s mounted read-only", loopdev);

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
    unlink(RUNTIME_IMAGE);
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

// ─── layers ─────────────────────────────────────────────────────────────────

static int valid(const char *s, const char *alphabet, size_t min, size_t max) {
    size_t n = strlen(s);
    return n >= min && n <= max && strspn(s, alphabet) == n;
}

// Runs /bin/sha256sum on path and compares the first 64 bytes it prints.
static int hash_matches(const char *path, const char *want) {
    int p[2];
    if (pipe2(p, O_CLOEXEC) != 0) return 0;
    pid_t pid = fork();
    if (pid < 0) return 0;
    if (pid == 0) {
        dup2(p[1], 1);
        char *argv[] = {"/bin/sha256sum", (char *)path, NULL};
        execv(argv[0], argv);
        _exit(127);
    }
    close(p[1]);
    char got[65] = {0};
    size_t n = 0;
    ssize_t r;
    while (n < 64 && (r = read(p[0], got + n, 64 - n)) > 0) n += (size_t)r;
    close(p[0]);
    int st = 0;
    while (waitpid(pid, &st, 0) < 0 && errno == EINTR) {}
    return n == 64 && WIFEXITED(st) && WEXITSTATUS(st) == 0 && strcmp(got, want) == 0;
}

// Mounts one layer read-only at /layers/.by-hash/<sha256> and points
// /layers/<name> at it. src is a block device or an image file.
static int mount_layer(const char *name, const char *sha, const char *src) {
    if (!valid(name, "abcdefghijklmnopqrstuvwxyz0123456789_-", 1, 32) ||
        !valid(sha, "0123456789abcdef", 64, 64)) {
        dprintf(2, "node-layer: bad name or sha256: %s %s\n", name, sha);
        return 1;
    }
    if (!hash_matches(src, sha)) {
        dprintf(2, "node-layer: %s: sha256 is not %s\n", src, sha);
        return 1;
    }
    struct stat st;
    if (stat(src, &st) != 0) die(src);
    char dev[32];
    const char *blk = src;
    if (S_ISREG(st.st_mode)) {
        attach_loop(src, dev, sizeof dev);
        blk = dev;
    }
    char dir[160], link[96], tmp[96], target[96];
    mkdir("/layers", 0755);
    mkdir("/layers/.by-hash", 0755);
    snprintf(dir, sizeof dir, "/layers/.by-hash/%s", sha);
    mkdir(dir, 0755);
    if (mount(blk, dir, "erofs", MS_RDONLY | MS_NODEV | MS_NOSUID, NULL) != 0 && errno != EBUSY) {
        dprintf(2, "node-layer: mount %s: %s\n", blk, strerror(errno));
        return 1;
    }
    snprintf(target, sizeof target, ".by-hash/%s", sha);
    snprintf(link, sizeof link, "/layers/%s", name);
    snprintf(tmp, sizeof tmp, "/layers/.%s.new", name);
    unlink(tmp);
    if (symlink(target, tmp) != 0 || rename(tmp, link) != 0) {
        dprintf(2, "node-layer: link %s: %s\n", link, strerror(errno));
        return 1;
    }
    dprintf(2, "node-layer: %s -> %s (%s)\n", link, target, src);
    return 0;
}

// Reads /sys/block/<dev>/serial into buf. Empty when there is none.
static void disk_serial(const char *dev, char *buf, size_t len) {
    char path[96];
    snprintf(path, sizeof path, "/sys/block/%s/serial", dev);
    buf[0] = 0;
    int fd = open(path, O_RDONLY | O_CLOEXEC);
    if (fd < 0) return;
    ssize_t n = read(fd, buf, len - 1);
    close(fd);
    buf[n > 0 ? n : 0] = 0;
    buf[strcspn(buf, "\n")] = 0;
}

// Finds the virtio disk with this serial. Returns 0 and fills dev, or -1.
static int find_disk(const char *serial, char *dev, size_t len) {
    DIR *d = opendir("/sys/block");
    if (!d) return -1;
    struct dirent *e;
    int found = -1;
    while ((e = readdir(d)) && found < 0) {
        if (strncmp(e->d_name, "vd", 2) != 0) continue;
        char s[64];
        disk_serial(e->d_name, s, sizeof s);
        if (strcmp(s, serial) == 0) {
            snprintf(dev, len, "/dev/%s", e->d_name);
            found = 0;
        }
    }
    closedir(d);
    return found;
}

// node_layer_<name>=<sha256> on the command line mounts disk "layer-<name>".
static void boot_layers(void) {
    extern char **environ;
    const char *prefix = "node_layer_";
    for (char **e = environ; *e; e++) {
        if (strncmp(*e, prefix, strlen(prefix)) != 0) continue;
        char name[64];
        const char *rest = *e + strlen(prefix);
        const char *eq = strchr(rest, '=');
        if (!eq || (size_t)(eq - rest) >= sizeof name) continue;
        memcpy(name, rest, (size_t)(eq - rest));
        name[eq - rest] = 0;
        char serial[80], dev[32];
        snprintf(serial, sizeof serial, "layer-%s", name);
        if (find_disk(serial, dev, sizeof dev) != 0) {
            say("no disk with serial %s", serial);
            continue;
        }
        mount_layer(name, eq + 1, dev);
    }
}

// ─── stage 2 ────────────────────────────────────────────────────────────────

static void loopback_up(void) {
    int s = socket(AF_INET, SOCK_DGRAM | SOCK_CLOEXEC, 0);
    if (s < 0) return;
    struct ifreq ifr;
    memset(&ifr, 0, sizeof ifr);
    strcpy(ifr.ifr_name, "lo");
    if (ioctl(s, SIOCGIFFLAGS, &ifr) == 0) {
        ifr.ifr_flags |= IFF_UP;
        ioctl(s, SIOCSIFFLAGS, &ifr);
    }
    close(s);
}

// dropbear in the foreground, logging to the console. Host keys are made on
// first use in /etc/dropbear, which is on the tmpfs overlay.
static pid_t start_sshd(void) {
    mkdir("/etc/dropbear", 0700);
    pid_t pid = fork();
    if (pid == 0) {
        char *argv[] = {"/sbin/dropbear", "-F", "-E", "-R", "-s", "-p", "22", NULL};
        execv(argv[0], argv);
        _exit(127);
    }
    return pid;
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
    char dev[32];
    if (find_disk("test", dev, sizeof dev) != 0) strcpy(dev, "/dev/vda");
    must_mount(dev, "/test", "erofs", MS_RDONLY, NULL);
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
        setenv("PATH", "/bin:/sbin", 1);
        char *argv[] = {"/test/entry", NULL};
        execv(argv[0], argv);
        dprintf(2, "node-init: exec /test/entry: %s\n", strerror(errno));
        _exit(127);
    }
    int st = 0;
    while (waitpid(pid, &st, 0) != pid) {}
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
    mkdir("/tmp", 01777);
    must_mount("tmpfs", "/tmp", "tmpfs", MS_NOSUID | MS_NODEV, "mode=1777");
    must_mount("devpts", "/dev/pts", "devpts", MS_NOSUID | MS_NOEXEC, "ptmxmode=0666,mode=0620");
    say("%s", "stage 2: runtime layer up, cgroup2 mounted");
    loopback_up();
    boot_layers();
    pid_t sshd = start_sshd();

    const char *t = getenv("node_test");
    if (t && strcmp(t, "1") == 0) return run_test();

    if (access("/etc/node/hook", X_OK) == 0) {
        if (fork() == 0) {
            char *argv[] = {"/etc/node/hook", NULL};
            execv(argv[0], argv);
            _exit(127);
        }
    } else {
        say("%s", "no /etc/node/hook; idle");
    }
    say("%s", "online");
    for (;;) {
        pid_t pid = wait(NULL);
        if (pid < 0 && errno == ECHILD) pause();
        if (pid == sshd) {
            say("%s", "dropbear exited; restarting");
            sleep(1);
            sshd = start_sshd();
        }
    }
}

int main(int argc, char **argv) {
    const char *base = strrchr(argv[0], '/');
    base = base ? base + 1 : argv[0];
    if (strcmp(base, "node-layer") == 0) {
        if (argc != 4) {
            dprintf(2, "usage: node-layer <name> <sha256> <image-or-device>\n");
            return 2;
        }
        return mount_layer(argv[1], argv[2], argv[3]);
    }
    if (getpid() != 1) {
        dprintf(2, "node-init: not PID 1\n");
        return 1;
    }
    if (strcmp(argv[0], "/init") == 0) return stage1();
    return stage2();
}
