// sshcheck runs inside the node VM, booted from the runtime layer alone.
// node_check on the kernel command line picks what it expects:
//
//	tpm        (default) node-identity serves the host key from the TPM;
//	           key login works, password login is refused, user and host
//	           certificates work, and the OS kept nothing from a past boot
//	no-tpm     no TPM: sshd is not listening and the FATAL line is in the
//	           kernel log
//	ephemeral  node_identity=ephemeral: a RAM host key, with the WARNING
//	reboot, poweroff  plain reboot or poweroff over SSH; the guest stops
//	button, cad  wait for vmrun's ACPI power button or Ctrl-Alt-Del
//
// It prints "hostkey: <key>" so vmrun can compare host keys across boots.
// Every SSH key here is made inside the VM by the pinned ssh-keygen, so no
// key material is a build input. Test keys go on the tmpfs overlay only.
package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	keys     = "/root/.ssh/authorized_keys"
	idDir    = "/run/node-identity"
	hostPub  = idDir + "/ssh_host_ecdsa_key.pub"
	fatalMsg = "identity: FATAL no TPM 2.0 (/dev/tpmrm0 missing): enable fTPM in BIOS (Security > AMD fTPM). sshd not started"
	warnMsg  = "identity: WARNING node_identity=ephemeral"
)

var (
	failed            int
	sshBin, keygenBin string
)

func main() {
	sshBin, _ = filepath.Abs("layers/openssh/ssh")
	keygenBin, _ = filepath.Abs("layers/openssh/ssh-keygen")
	mode := os.Getenv("node_check")
	if mode == "" {
		mode = "tpm"
	}
	fmt.Println("mode:", mode)
	runtimeChecks()
	switch mode {
	case "tpm":
		tpmChecks()
	case "no-tpm":
		noTPMChecks()
	case "ephemeral":
		ephemeralChecks()
	case "reboot", "poweroff":
		stopChecks(mode)
	case "button", "cad":
		// vmrun presses the ACPI power button or sends Ctrl-Alt-Del.
		waitListen("127.0.0.1:22")
		fmt.Println("ready-for-qmp")
		time.Sleep(120 * time.Second)
		bad("still running 120 s after the %s", mode)
	default:
		bad("unknown node_check %q", mode)
	}
	done()
}

func runtimeChecks() {
	for _, p := range []string{"/bin/go", "/bin/zig", "/bin/git", "/bin/nvim", "/bin/cc", "/usr/bin/cc", "/bin/python3", "/bin/zsh", "/bin/ssh", "/bin/ssh-keygen"} {
		if _, err := os.Lstat(p); err == nil {
			bad("runtime layer has %s", p)
		}
	}
	ok("runtime layer has no toolchains and no ssh client")
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
}

func tpmChecks() {
	if _, err := os.Stat("/dev/tpmrm0"); err != nil {
		bad("no /dev/tpmrm0: %v", err)
		done()
	}
	ok("/dev/tpmrm0 present")
	expectSource("tpm")
	// Stateless: nothing written on a past boot survives.
	const marker = "/var/lib/node-boot-marker"
	if _, err := os.Stat(marker); err == nil {
		bad("%s survived a reboot", marker)
	} else {
		os.MkdirAll(filepath.Dir(marker), 0o755)
		must(os.WriteFile(marker, []byte("x"), 0o644))
		ok("no state from a past boot")
	}
	waitListen("127.0.0.1:22")
	loginChecks()

	// The served key is the TPM's key.
	served := hostKeyFromKnownHosts()
	direct, err := exec.Command("/sbin/node-identity", "pubkey").Output()
	if err != nil {
		bad("node-identity pubkey: %v", err)
	} else if !strings.HasPrefix(string(direct), served+"\n") && !strings.HasPrefix(string(direct), served+" ") {
		bad("sshd presents %q, the TPM holds %q", served, firstLine(string(direct)))
	} else {
		ok("sshd's host key is the TPM key at handle 0x81008001")
	}
	fmt.Println("hostkey:", served)

	certChecks(served)
}

// stopChecks logs in and runs plain reboot or poweroff (toybox, which
// signals PID 1). vmrun passes the test when the guest stops cleanly.
func stopChecks(cmd string) {
	waitListen("127.0.0.1:22")
	must(keygen("-q", "-t", "ed25519", "-N", "", "-C", "test", "-f", "/tmp/id"))
	pub, err := os.ReadFile("/tmp/id.pub")
	must(err)
	appendFile(keys, pub)
	out, err := ssh("-i", "/tmp/id", "root@127.0.0.1", cmd)
	fmt.Printf("ssh root@127.0.0.1 %s: %v %s\n", cmd, err, strings.TrimSpace(out))
	time.Sleep(120 * time.Second)
	bad("still running 120 s after %s", cmd)
}

func noTPMChecks() {
	if _, err := os.Stat("/dev/tpmrm0"); err == nil {
		bad("this check wants no TPM, and /dev/tpmrm0 exists")
	}
	time.Sleep(3 * time.Second)
	if c, err := net.DialTimeout("tcp", "127.0.0.1:22", time.Second); err == nil {
		c.Close()
		bad("sshd is listening without a TPM")
	} else {
		ok("sshd is not listening")
	}
	if _, err := os.Stat(hostPub); err == nil {
		bad("%s exists without a TPM", hostPub)
	}
	if k := kmsg(); !strings.Contains(k, fatalMsg) {
		bad("the kernel log lacks %q", fatalMsg)
	} else {
		ok("kernel log: " + fatalMsg)
	}
}

func ephemeralChecks() {
	expectSource("ephemeral")
	if k := kmsg(); !strings.Contains(k, warnMsg) {
		bad("the kernel log lacks %q", warnMsg)
	} else {
		ok("kernel log has the ephemeral WARNING")
	}
	waitListen("127.0.0.1:22")
	loginChecks()
	fmt.Println("hostkey:", hostKeyFromKnownHosts())
}

// loginChecks: unknown key refused, password refused, key login, pty.
func loginChecks() {
	must(keygen("-q", "-t", "ed25519", "-N", "", "-C", "test", "-f", "/tmp/id"))
	if out, err := ssh("-i", "/tmp/id", "root@127.0.0.1", "echo should-not-run"); err == nil {
		bad("login with an unknown key worked: %s", out)
	} else {
		ok("unknown key refused")
	}
	out, err := ssh("-v", "-o", "PubkeyAuthentication=no",
		"-o", "PreferredAuthentications=password,keyboard-interactive", "root@127.0.0.1", "true")
	if err == nil {
		bad("a login without a key worked")
	} else if !strings.Contains(out, "Authentications that can continue: publickey\r\n") &&
		!strings.Contains(out, "Authentications that can continue: publickey\n") {
		bad("sshd offers more than publickey: %s", out)
	} else {
		ok("password and keyboard-interactive refused; sshd offers publickey only")
	}
	pub, err := os.ReadFile("/tmp/id.pub")
	must(err)
	appendFile(keys, pub)
	out, err = ssh("-i", "/tmp/id", "root@127.0.0.1", "echo ssh-ok; id -u; uname -r")
	if err != nil || !strings.HasPrefix(out, "ssh-ok\n0\n") {
		bad("key login: %v: %q", err, out)
	} else {
		ok("key login as root: " + strings.ReplaceAll(strings.TrimSpace(out), "\n", " "))
	}
	out, err = ssh("-tt", "-i", "/tmp/id", "root@127.0.0.1", "test -t 0 && echo tty-ok")
	if err != nil || !strings.Contains(out, "tty-ok") {
		bad("pty session: %v: %q", err, out)
	} else {
		ok("pty session")
	}
}

// certChecks: a user certificate from TrustedUserCAKeys logs in, and a host
// certificate from a sshd_config.d drop-in verifies against @cert-authority.
func certChecks(hostKey string) {
	must(keygen("-q", "-t", "ed25519", "-N", "", "-C", "test-ca", "-f", "/tmp/ca"))
	ca, err := os.ReadFile("/tmp/ca.pub")
	must(err)
	appendFile("/etc/ssh/trusted_user_ca_keys", ca)
	must(keygen("-q", "-t", "ed25519", "-N", "", "-C", "user", "-f", "/tmp/u"))
	must(keygen("-q", "-s", "/tmp/ca", "-I", "test-user", "-n", "root", "-V", "-5m:+1h", "/tmp/u.pub"))
	out, err := ssh("-i", "/tmp/u", "-o", "CertificateFile=/tmp/u-cert.pub", "root@127.0.0.1", "echo cert-ok")
	if err != nil || !strings.Contains(out, "cert-ok") {
		bad("user certificate login: %v: %q", err, out)
	} else {
		ok("user certificate from TrustedUserCAKeys logs in")
	}

	must(os.WriteFile("/tmp/host.pub", []byte(hostKey+"\n"), 0o644))
	must(keygen("-q", "-s", "/tmp/ca", "-h", "-I", "node", "-n", "127.0.0.1", "-V", "-5m:+1h", "/tmp/host.pub"))
	must(os.WriteFile("/etc/ssh/sshd_config.d/host-cert.conf", []byte("HostCertificate /tmp/host-cert.pub\n"), 0o644))
	d := exec.Command("/sbin/sshd", "-D", "-e", "-p", "2222", "-f", "/etc/ssh/sshd_config")
	var dlog bytes.Buffer
	d.Stderr = &dlog
	must(d.Start())
	defer func() { d.Process.Kill(); d.Wait() }()
	waitListen("127.0.0.1:2222")
	must(os.WriteFile("/tmp/kh-ca", append([]byte("@cert-authority 127.0.0.1,[127.0.0.1]:2222 "), ca...), 0o644))
	out, err = run(sshBin, "-F", "none", "-p", "2222", "-i", "/tmp/id",
		"-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile=/tmp/kh-ca",
		"-o", "HostKeyAlgorithms=ecdsa-sha2-nistp256-cert-v01@openssh.com",
		"root@127.0.0.1", "echo host-cert-ok")
	if err != nil || !strings.Contains(out, "host-cert-ok") {
		bad("host certificate: %v: %q\nsshd: %s", err, out, dlog.String())
	} else {
		ok("host certificate (drop-in HostCertificate, key still in the TPM) verifies via @cert-authority")
	}
}

func expectSource(want string) {
	b, err := os.ReadFile(idDir + "/source")
	if got := strings.TrimSpace(string(b)); err != nil || got != want {
		bad("identity source %q (%v), want %q", got, err, want)
	} else {
		ok("identity source: " + want)
	}
}

func hostKeyFromKnownHosts() string {
	b, _ := os.ReadFile("/tmp/.ssh/known_hosts")
	for _, l := range strings.Split(string(b), "\n") {
		f := strings.Fields(l)
		if len(f) >= 3 && f[1] == "ecdsa-sha2-nistp256" {
			return f[1] + " " + f[2]
		}
	}
	bad("no ecdsa host key in known_hosts: %q", b)
	return ""
}

func ssh(args ...string) (string, error) {
	base := []string{"-F", "none", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=/tmp/.ssh/known_hosts", "-o", "ConnectTimeout=10"}
	return run(sshBin, append(base, args...)...)
}

func keygen(args ...string) error {
	out, err := run(keygenBin, args...)
	if err != nil {
		return fmt.Errorf("ssh-keygen %v: %v: %s", args, err, out)
	}
	return nil
}

func run(name string, args ...string) (string, error) {
	os.MkdirAll("/tmp/.ssh", 0o700)
	c := exec.Command(name, args...)
	c.Env = []string{"HOME=/tmp", "PATH=/bin:/sbin", "TERM=dumb"}
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	err := c.Run()
	return strings.ReplaceAll(b.String(), "\r\n", "\n"), err
}

func waitListen(addr string) {
	deadline := time.Now().Add(30 * time.Second)
	for {
		c, err := net.Dial("tcp", addr)
		if err == nil {
			c.Close()
			ok("sshd listens on " + addr)
			return
		}
		if time.Now().After(deadline) {
			bad("nothing listens on %s: %v", addr, err)
			done()
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// kmsg reads the whole kernel log without blocking.
func kmsg() string {
	fd, err := syscall.Open("/dev/kmsg", syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return ""
	}
	defer syscall.Close(fd)
	var all strings.Builder
	buf := make([]byte, 8192)
	for {
		n, err := syscall.Read(fd, buf)
		if err == syscall.EPIPE {
			continue
		}
		if err != nil || n <= 0 {
			return all.String()
		}
		all.Write(buf[:n])
	}
}

func appendFile(path string, b []byte) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	must(err)
	_, err = f.Write(b)
	must(err)
	must(f.Close())
}

func firstLine(s string) string { l, _, _ := strings.Cut(s, "\n"); return l }

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
