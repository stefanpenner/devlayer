// sshcheck runs inside the node VM, booted from the runtime layer alone.
// It checks that the layer holds no toolchains, that Stefan's pinned keys are
// root's authorized_keys, and that dropbear takes a public-key login and
// refuses one it does not know.
//
// The test key is made at build time by the pinned dropbearkey. It lives only
// on this test's disk. The test appends its public half to authorized_keys,
// which sits on the tmpfs overlay, so the runtime layer never holds it.
package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	multi = "/bin/dropbearmulti"
	keys  = "/root/.ssh/authorized_keys"
)

var failed int

func main() {
	for _, p := range []string{"/bin/go", "/bin/zig", "/bin/git", "/bin/nvim", "/bin/cc", "/usr/bin/cc", "/bin/python3", "/bin/zsh"} {
		if _, err := os.Lstat(p); err == nil {
			bad("runtime layer has %s", p)
		}
	}
	ok("runtime layer has no toolchains")

	st, err := os.Stat(keys)
	if err != nil {
		bad("%v", err)
	} else if st.Mode().Perm() != 0o600 {
		bad("%s mode %v", keys, st.Mode().Perm())
	}
	pinned, _ := os.ReadFile(keys)
	n := 0
	for _, l := range strings.Split(string(pinned), "\n") {
		if strings.HasPrefix(l, "ssh-") || strings.HasPrefix(l, "ecdsa-") {
			n++
		}
	}
	if n == 0 {
		bad("%s has no keys", keys)
	} else {
		ok(fmt.Sprintf("%d pinned keys in %s", n, keys))
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		c, err := net.Dial("tcp", "127.0.0.1:22")
		if err == nil {
			c.Close()
			break
		}
		if time.Now().After(deadline) {
			bad("sshd never listened: %v", err)
			done()
		}
		time.Sleep(200 * time.Millisecond)
	}
	ok("dropbear listens on :22")

	key, err := os.ReadFile("node/sshcheck/test_key")
	must(err)
	must(os.WriteFile("/tmp/id", key, 0o600))

	if out, err := login("echo should-not-run"); err == nil {
		bad("login with an unknown key worked: %s", out)
	} else {
		ok("unknown key refused")
	}

	pub, err := exec.Command(multi, "dropbearkey", "-y", "-f", "/tmp/id").Output()
	must(err)
	line := ""
	for _, l := range strings.Split(string(pub), "\n") {
		if strings.HasPrefix(l, "ssh-") {
			line = l
		}
	}
	if line == "" {
		bad("no public key from dropbearkey: %s", pub)
		done()
	}
	f, err := os.OpenFile(keys, os.O_APPEND|os.O_WRONLY, 0)
	must(err)
	fmt.Fprintln(f, line)
	must(f.Close())

	out, err := login("echo ssh-ok; id -u; uname -r")
	if err != nil || !strings.HasPrefix(out, "ssh-ok\n0\n") {
		bad("key login: %v: %q", err, out)
	} else {
		ok("key login as root: " + strings.ReplaceAll(strings.TrimSpace(out), "\n", " "))
	}

	// A pty session, as a person gets with plain `ssh node`. dbclient
	// wants a terminal of its own, so it runs on a local pty.
	out, err = onPty(multi, "dbclient", "-y", "-t", "-i", "/tmp/id", "root@127.0.0.1", "test -t 0 && echo tty-ok")
	if err != nil || !strings.Contains(out, "tty-ok") {
		bad("pty session: %v: %q", err, out)
	} else {
		ok("pty session")
	}
	done()
}

func login(cmd string) (string, error) {
	return run(multi, "dbclient", "-y", "-i", "/tmp/id", "root@127.0.0.1", cmd)
}

func run(name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	c.Env = []string{"HOME=/tmp", "PATH=/bin:/sbin"}
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	err := c.Run()
	return b.String(), err
}

// onPty runs a command with a new pty as its controlling terminal and
// returns what it wrote there.
func onPty(name string, args ...string) (string, error) {
	const tiocgptn, tiocsptlck = 0x80045430, 0x40045431
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	defer m.Close()
	var n, unlock uint32
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, m.Fd(), tiocsptlck, uintptr(unsafe.Pointer(&unlock))); e != 0 {
		return "", e
	}
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, m.Fd(), tiocgptn, uintptr(unsafe.Pointer(&n))); e != 0 {
		return "", e
	}
	sl, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return "", err
	}
	c := exec.Command(name, args...)
	c.Env = []string{"HOME=/tmp", "PATH=/bin:/sbin", "TERM=vt100"}
	c.Stdin, c.Stdout, c.Stderr = sl, sl, sl
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	var b bytes.Buffer
	copied := make(chan struct{})
	go func() { io.Copy(&b, m); close(copied) }()
	err = c.Run()
	sl.Close()
	select {
	case <-copied:
	case <-time.After(2 * time.Second):
	}
	return b.String(), err
}

func ok(s string) { fmt.Println("ok  ", s) }

func bad(f string, a ...any) {
	failed++
	fmt.Printf("FAIL "+f+"\n", a...)
}

func must(err error) {
	if err != nil {
		bad("%v", err)
		done()
	}
}

func done() {
	if failed > 0 {
		os.Exit(1)
	}
	os.Exit(0)
}
