// vmrun is the test executable of a node_vm_test. It boots the node kernel
// and initrd in the pinned QEMU, with the test image as the virtio disk with
// serial "test", and node_test=1 on the command line. Init runs /test/entry from that
// disk. The payload's stdout and stderr come back on two virtio consoles.
// Its exit status comes back through isa-debug-exit: QEMU exits with
// (status << 1) | 1.
//
// Environment, set by the rule (paths relative to the runfiles tree):
//
//	NODE_QEMU NODE_KERNEL NODE_INITRD NODE_DISK
//	NODE_LAYERS   name=path,... extra EROFS layers, may be empty
//
// Each layer is a read-only disk with serial "layer-<name>", and the command
// line gets node_layer_<name>=<sha256 of the image>. Init mounts it at
// /layers/<name> only when the bytes match.
//
//	NODE_SWTPM            swtpm path; set means the guest gets a TPM 2.0 (tpm-crb)
//	NODE_BOOTS            boots of the same disks and TPM state, default 1
//	NODE_CMDLINE          more kernel command line
//	NODE_EXPECT_SERIAL    text every boot log must contain
//	NODE_HOSTKEY_ACROSS_BOOTS  "same" or "differ": compares the payload's
//	                      "hostkey: " stdout line across boots
//	NODE_EXPECT_STOP      "reboot" or "poweroff": the guest is expected to
//	                      stop through init's shutdown, not report a status.
//	                      The test passes when QEMU exits cleanly and the
//	                      serial log has init's shutdown line and the
//	                      kernel's "Restarting system" or "Power down".
//	NODE_QMP_ON, NODE_QMP_CMD  when the payload's stdout has NODE_QMP_ON,
//	                      send the QMP command NODE_QMP_CMD (JSON), for
//	                      example system_powerdown (the ACPI power button)
//
// The TPM state directory lives in TEST_TMPDIR for the whole test, so every
// boot sees the same TPM, as a real node does. The guest keeps nothing else.
//
// NODE_VM_ACCEL picks kvm, hvf, or tcg. Empty means kvm when /dev/kvm opens,
// hvf on an Intel Mac, and tcg otherwise. Apple silicon has no x86 HVF.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
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

	accel := os.Getenv("NODE_VM_ACCEL")
	if accel == "" {
		accel = pickAccel()
	}
	cpu := "max"
	if accel == "kvm" || accel == "hvf" {
		cpu = "host"
	}
	appendArgs := "console=ttyS0 ip=dhcp panic=-1 node_test=1 node_verbose=1"
	if v := os.Getenv("NODE_CMDLINE"); v != "" {
		appendArgs += " " + v
	}
	var layerArgs []string
	if v := os.Getenv("NODE_LAYERS"); v != "" {
		for _, l := range strings.Split(v, ",") {
			name, path, ok := strings.Cut(l, "=")
			if !ok {
				fail(fmt.Errorf("NODE_LAYERS entry %q", l))
			}
			path = mustAbs(path)
			sum, err := fileSHA256(path)
			check(err)
			appendArgs += " node_layer_" + name + "=" + sum
			layerArgs = append(layerArgs,
				"-drive", "if=none,id=l"+name+",format=raw,readonly=on,file="+path,
				"-device", "virtio-blk-pci,drive=l"+name+",serial=layer-"+name)
		}
	}
	swtpm := os.Getenv("NODE_SWTPM")
	tpmState := filepath.Join(dir, "tpm-state")
	if swtpm != "" {
		swtpm = mustAbs(swtpm)
		check(os.MkdirAll(tpmState, 0o700))
	}
	boots := 1
	if v := os.Getenv("NODE_BOOTS"); v != "" {
		_, err := fmt.Sscan(v, &boots)
		check(err)
	}

	var hostKeys []string
	status := 0
	for n := 1; n <= boots; n++ {
		bootDir := filepath.Join(dir, fmt.Sprintf("boot%d", n))
		check(os.MkdirAll(bootDir, 0o755))
		args := baseArgs(qemu, accel, cpu, kernel, initrd, disk, appendArgs, bootDir)
		args = append(args, layerArgs...)
		if os.Getenv("NODE_DISPLAY") != "" {
			args = withDisplay(qemu, args)
		}
		var tpm *exec.Cmd
		if swtpm != "" {
			// A unix socket path is capped at 108 bytes; bazel's TEST_TMPDIR is
			// longer, so the socket lives in a short directory under /tmp.
			sockDir, err := os.MkdirTemp("/tmp", "swtpm")
			check(err)
			defer os.RemoveAll(sockDir)
			sock := filepath.Join(sockDir, "s")
			tpm = hostCommand(swtpm, []string{"socket", "--tpm2", "--terminate",
				"--tpmstate", "dir=" + tpmState,
				"--ctrl", "type=unixio,path=" + sock})
			tpm.Stderr = os.Stderr
			check(tpm.Start())
			waitFor(sock)
			args = append(args,
				"-chardev", "socket,id=chrtpm,path="+sock,
				"-tpmdev", "emulator,id=tpm0,chardev=chrtpm",
				"-device", "tpm-crb,tpmdev=tpm0")
		}
		code, out := boot(qemu, args, bootDir, accel, n, boots)
		if tpm != nil {
			tpm.Process.Kill()
			tpm.Wait()
		}
		if v := os.Getenv("NODE_EXPECT_SERIAL"); v != "" {
			b, _ := os.ReadFile(filepath.Join(bootDir, "boot.log"))
			if !strings.Contains(string(b), v) {
				fmt.Fprintf(os.Stderr, "vmrun: boot %d: the serial log lacks %q\n", n, v)
				code = 1
			} else {
				fmt.Fprintf(os.Stderr, "vmrun: boot %d: serial has %q\n", n, v)
			}
		}
		for _, l := range strings.Split(out, "\n") {
			if k, ok := strings.CutPrefix(l, "hostkey: "); ok {
				hostKeys = append(hostKeys, k)
			}
		}
		if code != 0 {
			status = code
			break
		}
	}
	if mode := os.Getenv("NODE_HOSTKEY_ACROSS_BOOTS"); mode != "" && status == 0 {
		if len(hostKeys) != boots {
			fail(fmt.Errorf("want one hostkey line per boot, got %d", len(hostKeys)))
		}
		same := true
		for _, k := range hostKeys[1:] {
			same = same && k == hostKeys[0]
		}
		switch {
		case mode == "same" && !same:
			fail(fmt.Errorf("the host key changed across boots: %q", hostKeys))
		case mode == "differ" && same:
			fail(fmt.Errorf("the host key stayed the same across boots: %q", hostKeys[0]))
		}
		fmt.Fprintf(os.Stderr, "vmrun: host key across %d boots: %s, as expected\n", boots, mode)
	}
	os.Exit(status)
}

func baseArgs(qemu, accel, cpu, kernel, initrd, disk, appendArgs, bootDir string) []string {
	share := filepath.Join(filepath.Dir(filepath.Dir(qemu)), "share", "qemu")
	return []string{
		"-L", share,
		"-machine", "q35", "-accel", accel, "-cpu", cpu, "-smp", "2", "-m", "2048",
		"-display", "none", "-vga", "none", "-monitor", "none", "-no-reboot",
		"-serial", "file:" + filepath.Join(bootDir, "boot.log"),
		"-kernel", kernel, "-initrd", initrd,
		"-append", appendArgs,
		"-netdev", "user,id=n0", "-device", "virtio-net-pci,netdev=n0,romfile=",
		"-drive", "if=none,id=t,format=raw,readonly=on,file=" + disk,
		"-device", "virtio-blk-pci,drive=t,serial=test",
		"-device", "virtio-serial-pci",
		"-chardev", "file,id=o,path=" + filepath.Join(bootDir, "stdout"), "-device", "virtconsole,chardev=o",
		"-chardev", "file,id=e,path=" + filepath.Join(bootDir, "stderr"), "-device", "virtconsole,chardev=e",
		"-device", "isa-debug-exit,iobase=0xf4,iosize=0x04",
	}
}

// boot runs one QEMU and returns the payload status and its stdout.
func boot(qemu string, args []string, bootDir, accel string, n, boots int) (int, string) {
	var qmpSock string
	if os.Getenv("NODE_QMP_ON") != "" {
		d, err := os.MkdirTemp("/tmp", "qmp")
		check(err)
		defer os.RemoveAll(d)
		qmpSock = filepath.Join(d, "s")
		args = append(args, "-qmp", "unix:"+qmpSock+",server,nowait")
	}
	cmd := hostCommand(qemu, args)
	var qerr strings.Builder
	cmd.Stderr = &qerr
	start := time.Now()
	check(cmd.Start())
	if qmpSock != "" {
		go qmpWhen(filepath.Join(bootDir, "stdout"), os.Getenv("NODE_QMP_ON"), qmpSock, os.Getenv("NODE_QMP_CMD"))
	}
	runErr := cmd.Wait()
	wall := time.Since(start)

	bootPath := filepath.Join(bootDir, "boot.log")
	out, _ := os.ReadFile(filepath.Join(bootDir, "stdout"))
	if boots > 1 {
		fmt.Printf("--- boot %d ---\n", n)
	}
	os.Stdout.Write(out)
	copyTo(os.Stderr, filepath.Join(bootDir, "stderr"))
	if d := os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR"); d != "" {
		if b, err := os.ReadFile(bootPath); err == nil {
			name := "boot.log"
			if boots > 1 {
				name = fmt.Sprintf("boot%d.log", n)
			}
			os.WriteFile(filepath.Join(d, name), b, 0o644)
		}
	}
	fmt.Fprintf(os.Stderr, "vmrun: boot %d: accel=%s wall=%.2fs\n", n, accel, wall.Seconds())

	var ee *exec.ExitError
	switch {
	case runErr == nil && os.Getenv("NODE_EXPECT_STOP") != "":
		return expectStop(os.Getenv("NODE_EXPECT_STOP"), bootPath), string(out)
	case runErr == nil:
		// Exit 0 means a power-off or reset without isa-debug-exit.
		fail(fmt.Errorf("the guest stopped before the payload reported a status\n%s", tail(bootPath)))
	case errors.As(runErr, &ee) && ee.ExitCode()%2 == 1:
		return (ee.ExitCode() - 1) / 2, string(out)
	default:
		fail(fmt.Errorf("qemu: %v: %s\n%s", runErr, qerr.String(), tail(bootPath)))
	}
	return 1, string(out)
}

// expectStop checks a clean guest stop: init's shutdown line, then the
// kernel's restart or power-down line. -no-reboot turns a reboot into a
// QEMU exit.
func expectStop(want, bootPath string) int {
	b, _ := os.ReadFile(bootPath)
	log := string(b)
	kernel := map[string]string{"reboot": "reboot: Restarting system", "poweroff": "reboot: Power down"}[want]
	for _, s := range []string{"node-init: stopping every process", kernel} {
		if s == "" || !strings.Contains(log, s) {
			fmt.Fprintf(os.Stderr, "vmrun: the guest stopped, but the serial log lacks %q\n%s\n", s, tail(bootPath))
			return 1
		}
	}
	fmt.Fprintf(os.Stderr, "vmrun: clean %s: serial has init's shutdown line and %q\n", want, kernel)
	return 0
}

// withDisplay boots through OVMF with a std VGA, so the kernel gets efifb
// and fbcon draws tty0 like on the SER9 (for screendumps).
func withDisplay(qemu string, args []string) []string {
	share := filepath.Join(filepath.Dir(filepath.Dir(qemu)), "share", "qemu")
	for i := range args {
		if args[i] == "-vga" && i+1 < len(args) {
			args[i+1] = "std"
		}
	}
	return append(args, "-drive", "if=pflash,format=raw,readonly=on,file="+filepath.Join(share, "edk2-x86_64-code.fd"))
}

// qmpWhen waits until path contains text, then sends one QMP command.
// $OUT in the command is the test's undeclared outputs directory.
func qmpWhen(path, text, sock, command string) {
	for i := 0; i < 6000; i++ {
		if b, err := os.ReadFile(path); err == nil && strings.Contains(string(b), text) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	c, err := net.Dial("unix", sock)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vmrun: qmp:", err)
		return
	}
	defer c.Close()
	r := bufio.NewReader(c)
	r.ReadString('\n') // greeting
	fmt.Fprintln(c, `{"execute":"qmp_capabilities"}`)
	r.ReadString('\n')
	command = strings.ReplaceAll(command, "$OUT", os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR"))
	fmt.Fprintln(c, command)
	resp, _ := r.ReadString('\n')
	fmt.Fprintf(os.Stderr, "vmrun: qmp %s -> %s", command, resp)
}

func waitFor(path string) {
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	fail(fmt.Errorf("%s never appeared", path))
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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

// hostCommand starts an Alpine program (QEMU, swtpm) with its own musl
// loader, or a Homebrew one with its bottle libraries. The host libc and host
// libraries are not used.
func hostCommand(qemu string, args []string) *exec.Cmd {
	root := filepath.Dir(filepath.Dir(filepath.Dir(qemu)))
	loaders, _ := filepath.Glob(filepath.Join(root, "lib", "ld-musl-*.so.1"))
	if len(loaders) == 1 {
		libs := filepath.Join(root, "lib") + ":" + filepath.Join(root, "usr", "lib") + ":" +
			filepath.Join(root, "usr", "lib", "swtpm") // libswtpm_libtpms.so.0
		cmd := exec.Command(loaders[0], append([]string{"--library-path", libs, qemu}, args...)...)
		cmd.Env = []string{}
		return cmd
	}
	cellar := filepath.Dir(filepath.Dir(filepath.Dir(qemu)))
	dirs, _ := filepath.Glob(filepath.Join(cellar, "*", "*", "lib"))
	more, _ := filepath.Glob(filepath.Join(cellar, "*", "*", "lib", "swtpm"))
	dirs = append(dirs, more...)
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
