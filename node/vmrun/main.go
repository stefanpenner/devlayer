// vmrun is the test executable of a node_vm_test. It boots the node kernel
// and initrd in the pinned QEMU, with the test image as the first virtio
// disk, and node_test=1 on the command line. Init runs /test/entry from that
// disk. The payload's stdout and stderr come back on two virtio consoles.
// Its exit status comes back through isa-debug-exit: QEMU exits with
// (status << 1) | 1.
//
// Environment, set by the rule (paths relative to the runfiles tree):
//
//	NODE_QEMU NODE_KERNEL NODE_INITRD NODE_DISK
//
// NODE_VM_ACCEL picks kvm, hvf, or tcg. Empty means kvm when /dev/kvm opens,
// hvf on an Intel Mac, and tcg otherwise. Apple silicon has no x86 HVF.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	qemu := mustAbs(env("NODE_QEMU"))
	kernel := mustAbs(env("NODE_KERNEL"))
	initrd := mustAbs(env("NODE_INITRD"))
	disk := mustAbs(env("NODE_DISK"))

	dir, err := os.MkdirTemp(os.Getenv("TEST_TMPDIR"), "vm")
	check(err)
	outPath := filepath.Join(dir, "stdout")
	errPath := filepath.Join(dir, "stderr")
	bootPath := filepath.Join(dir, "boot.log")

	accel := os.Getenv("NODE_VM_ACCEL")
	if accel == "" {
		accel = pickAccel()
	}
	cpu := "max"
	if accel == "kvm" || accel == "hvf" {
		cpu = "host"
	}
	share := filepath.Join(filepath.Dir(filepath.Dir(qemu)), "share", "qemu")
	args := []string{
		"-L", share,
		"-machine", "q35", "-accel", accel, "-cpu", cpu, "-smp", "2", "-m", "2048",
		"-display", "none", "-vga", "none", "-monitor", "none", "-no-reboot",
		"-serial", "file:" + bootPath,
		"-kernel", kernel, "-initrd", initrd,
		"-append", "console=ttyS0 ip=dhcp panic=-1 node_test=1",
		"-netdev", "user,id=n0", "-device", "virtio-net-pci,netdev=n0,romfile=",
		"-drive", "if=none,id=t,format=raw,readonly=on,file=" + disk,
		"-device", "virtio-blk-pci,drive=t",
		"-device", "virtio-serial-pci",
		"-chardev", "file,id=o,path=" + outPath, "-device", "virtconsole,chardev=o",
		"-chardev", "file,id=e,path=" + errPath, "-device", "virtconsole,chardev=e",
		"-device", "isa-debug-exit,iobase=0xf4,iosize=0x04",
	}
	cmd := qemuCommand(qemu, args)
	var qerr strings.Builder
	cmd.Stderr = &qerr
	start := time.Now()
	runErr := cmd.Run()
	wall := time.Since(start)

	copyTo(os.Stdout, outPath)
	copyTo(os.Stderr, errPath)
	if d := os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR"); d != "" {
		if b, err := os.ReadFile(bootPath); err == nil {
			os.WriteFile(filepath.Join(d, "boot.log"), b, 0o644)
		}
	}
	fmt.Fprintf(os.Stderr, "vmrun: accel=%s wall=%.2fs\n", accel, wall.Seconds())

	var ee *exec.ExitError
	switch {
	case runErr == nil:
		// Exit 0 means a power-off or reset without isa-debug-exit.
		fail(fmt.Errorf("the guest stopped before the payload reported a status\n%s", tail(bootPath)))
	case errors.As(runErr, &ee) && ee.ExitCode()%2 == 1:
		os.Exit((ee.ExitCode() - 1) / 2)
	default:
		fail(fmt.Errorf("qemu: %v: %s\n%s", runErr, qerr.String(), tail(bootPath)))
	}
}

func pickAccel() string {
	switch runtime.GOOS {
	case "linux":
		if f, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0); err == nil {
			f.Close()
			return "kvm"
		}
	case "darwin":
		if runtime.GOARCH == "amd64" {
			return "hvf"
		}
	}
	return "tcg"
}

// qemuCommand starts an Alpine QEMU with its own musl loader, or a Homebrew
// QEMU with its bottle libraries. The host libc and host libraries are not used.
func qemuCommand(qemu string, args []string) *exec.Cmd {
	root := filepath.Dir(filepath.Dir(filepath.Dir(qemu)))
	loaders, _ := filepath.Glob(filepath.Join(root, "lib", "ld-musl-*.so.1"))
	if len(loaders) == 1 {
		libs := filepath.Join(root, "lib") + ":" + filepath.Join(root, "usr", "lib")
		cmd := exec.Command(loaders[0], append([]string{"--library-path", libs, qemu}, args...)...)
		cmd.Env = []string{}
		return cmd
	}
	cellar := filepath.Dir(filepath.Dir(filepath.Dir(qemu)))
	dirs, _ := filepath.Glob(filepath.Join(cellar, "*", "*", "lib"))
	cmd := exec.Command(qemu, args...)
	cmd.Env = []string{"DYLD_LIBRARY_PATH=" + strings.Join(dirs, ":")}
	return cmd
}

func copyTo(w io.Writer, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	io.Copy(w, f)
}

func tail(path string) string {
	b, _ := os.ReadFile(path)
	if len(b) > 6000 {
		b = b[len(b)-6000:]
	}
	return string(b)
}

func env(k string) string {
	v := os.Getenv(k)
	if v == "" {
		fail(fmt.Errorf("%s is not set", k))
	}
	return v
}

func mustAbs(p string) string {
	a, err := filepath.Abs(p)
	check(err)
	return a
}

func check(err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "vmrun:", err)
	os.Exit(1)
}
