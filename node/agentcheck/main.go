// agentcheck runs inside the node VM with a TPM (swtpm). It checks
// node-agent from the runtime layer:
//
//   - the banner shows the address, the TPM, the host key, sshd and the ssh
//     command, and the host key file is 0600 (sshd's own permission check);
//   - the release watcher, against an HTTPS server on 127.0.0.1 with a CA
//     made here: the same release does not reboot, a fetch error does not,
//     a corrupt blob does not, a new release does (a test marker stands in
//     for reboot(2)), and the loop guard in the TPM NV index holds back a
//     second reboot for the same target until the retry wait has passed.
//
// node_check=optout (with node_autoreboot=off on the command line) checks
// that the watcher never reboots.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const agent = "/sbin/node-agent"

var failed int

func ok(f string, a ...any)  { fmt.Printf("ok   "+f+"\n", a...) }
func bad(f string, a ...any) { failed++; fmt.Printf("FAIL "+f+"\n", a...) }

func must(err error) {
	if err != nil {
		fmt.Println("FAIL", err)
		os.Exit(1)
	}
}

// ─── the test server ────────────────────────────────────────────────────────

type server struct {
	mu      sync.Mutex
	files   map[string][]byte
	notMod  int
	failIdx bool
	url     string
}

func etag(b []byte) string {
	h := sha256.Sum256(b)
	return `"` + hex.EncodeToString(h[:8]) + `"`
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "index.json" && s.failIdx {
		http.Error(w, "boom", 500)
		return
	}
	b, found := s.files[p]
	if !found {
		http.NotFound(w, r)
		return
	}
	e := etag(b)
	if r.Header.Get("If-None-Match") == e {
		s.notMod++
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", e)
	w.Write(b)
}

func digest(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

func (s *server) put(b []byte) string {
	d := digest(b)
	s.files["blobs/sha256/"+strings.TrimPrefix(d, "sha256:")] = b
	return d
}

type desc struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int               `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// publish makes a release and points index.json at it. corrupt serves the
// initrd with bytes that do not match its digest. It returns the manifest
// digest.
func (s *server) publish(version string, corrupt bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	kernel := make([]byte, 3<<20)
	initrd := make([]byte, 1<<20)
	rand.Read(kernel)
	rand.Read(initrd)
	kd, id := s.put(kernel), s.put(initrd)
	if corrupt {
		bad := append([]byte(nil), initrd...)
		bad[100] ^= 0xff
		s.files["blobs/sha256/"+strings.TrimPrefix(id, "sha256:")] = bad
	}
	cfg := []byte("{}")
	cd := s.put(cfg)
	cmdline := "console=ttyS0 console=tty0 ip=dhcp node_release=" + version
	m, _ := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.manifest.v1+json",
		"artifactType":  "application/vnd.org.pulpproject.netboot.artifact.v1",
		"config":        desc{MediaType: "application/vnd.oci.empty.v1+json", Digest: cd, Size: len(cfg)},
		"layers": []desc{
			{MediaType: "application/x-netboot-file", Digest: kd, Size: len(kernel), Annotations: map[string]string{"org.opencontainers.image.title": "vmlinuz"}},
			{MediaType: "application/x-netboot-file", Digest: id, Size: len(initrd), Annotations: map[string]string{"org.opencontainers.image.title": "initrd.img"}},
		},
		"annotations": map[string]string{"org.opencontainers.image.version": version, "network.penner.boot.cmdline": cmdline},
	})
	md := s.put(m)
	idx, _ := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.index.v1+json",
		"manifests": []desc{{
			MediaType:   "application/vnd.oci.image.manifest.v1+json",
			Digest:      md,
			Size:        len(m),
			Annotations: map[string]string{"org.opencontainers.image.version": version, "network.penner.boot.cmdline": cmdline},
		}},
	})
	s.files["index.json"] = idx
	return md
}

// startServer makes a CA and a 127.0.0.1 certificate, writes the CA to
// /tmp/ca.pem, and serves on a free port.
func startServer() *server {
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "agentcheck test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	must(err)
	caCert, _ := x509.ParseCertificate(caDER)
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, KeyUsage: x509.KeyUsageDigitalSignature,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, caCert, &leafKey.PublicKey, caKey)
	must(err)
	must(os.WriteFile("/tmp/ca.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o644))
	l, err := net.Listen("tcp", "127.0.0.1:0")
	must(err)
	s := &server{files: map[string][]byte{}, url: "https://" + l.Addr().String() + "/index.json"}
	tl := tls.NewListener(l, &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{leafDER}, PrivateKey: leafKey}}})
	go http.Serve(tl, s)
	return s
}

// ─── running the agent ──────────────────────────────────────────────────────

const marker = "/tmp/reboot-marker"

// watch runs node-agent watch as release `current` and returns its log and
// the reboot marker ("" when it did not reboot).
func watch(s *server, current string, checks int, extra ...string) (string, string) {
	os.Remove(marker)
	args := append([]string{"watch", "-index", s.url, "-roots", "/tmp/ca.pem",
		"-interval", "200ms", "-grace", "100ms", "-fail-wait", "200ms", "-early", "0s",
		"-checks", fmt.Sprint(checks), "-reboot-marker", marker, "-efivars", "/tmp/no-efivars"}, extra...)
	cmd := exec.Command(agent, args...)
	cmd.Env = append(os.Environ(), "node_release="+current)
	out, err := cmd.CombinedOutput()
	if err != nil {
		bad("node-agent watch: %v\n%s", err, out)
	}
	m, _ := os.ReadFile(marker)
	return string(out), strings.TrimSpace(string(m))
}

type record struct {
	Target string `json:"target"`
	Tries  int    `json:"tries"`
}

func readRecord() record {
	out, err := exec.Command(agent, "record").CombinedOutput()
	var r record
	if err != nil {
		bad("node-agent record: %v %s", err, out)
		return r
	}
	json.Unmarshal(out, &r)
	return r
}

func expect(name, log, mark, wantMark, wantLog string) {
	switch {
	case mark != wantMark:
		bad("%s: reboot marker %q, want %q\n%s", name, mark, wantMark, log)
	case !strings.Contains(log, wantLog):
		bad("%s: log lacks %q\n%s", name, wantLog, log)
	default:
		if wantMark == "" {
			ok("%s: no reboot (%s)", name, wantLog)
		} else {
			ok("%s: reboot for %s", name, wantMark[:19])
		}
	}
}

func bannerChecks() {
	// sshd and the identity may still be starting.
	var out []byte
	for i := 0; i < 100; i++ {
		out, _ = exec.Command(agent, "banner").CombinedOutput()
		if strings.Contains(string(out), "listening on port 22") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	b := string(out)
	fmt.Print(b)
	for _, want := range []string{
		"10.0.2.15/24 via 10.0.2.2", "tpm      yes", "host key SHA256:", "(tpm)",
		"sshd     listening on port 22", "ssh root@10.0.2.15", "auto-reboot off (no node_release",
	} {
		if strings.Contains(b, want) {
			ok("banner shows %q", want)
		} else {
			bad("banner lacks %q", want)
		}
	}
	if fi, err := os.Stat("/run/node-identity/ssh_host_ecdsa_key.pub"); err != nil || fi.Mode().Perm() != 0o600 {
		bad("host key file mode: %v %v", fi, err)
	} else {
		ok("host key file is 0600, so sshd does not warn")
	}
}

// kmsg returns the kernel log records as "level message".
func kmsg() []string {
	fd, err := syscall.Open("/dev/kmsg", syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		bad("open /dev/kmsg: %v", err)
		return nil
	}
	defer syscall.Close(fd)
	var recs []string
	buf := make([]byte, 8192)
	for {
		n, err := syscall.Read(fd, buf)
		if err == syscall.EPIPE {
			continue
		}
		if err != nil || n <= 0 {
			return recs
		}
		head, msg, _ := strings.Cut(string(buf[:n]), ";")
		pri, _ := strconv.Atoi(strings.Split(head, ",")[0])
		recs = append(recs, fmt.Sprintf("%d %s", pri&7, strings.SplitN(msg, "\n", 2)[0]))
	}
}

// sshd logs go to the kernel log at debug level (not the console), and
// sshd does not complain about the host key file.
func sshdLogChecks() {
	if c, err := net.Dial("tcp", "127.0.0.1:22"); err == nil {
		c.Write([]byte("SSH-2.0-agentcheck\r\n"))
		time.Sleep(300 * time.Millisecond)
		c.Close()
	}
	time.Sleep(500 * time.Millisecond)
	debug, loud := 0, 0
	for _, r := range kmsg() {
		lvl, msg, _ := strings.Cut(r, " ")
		if strings.Contains(msg, "UNPROTECTED") || strings.Contains(msg, "too open") {
			bad("sshd warns about the host key file: %s", msg)
		}
		if strings.HasPrefix(msg, "sshd: ") {
			if lvl == "7" {
				debug++
			} else if !strings.Contains(msg, "error:") && !strings.Contains(msg, "fatal:") {
				loud++
				bad("sshd line above debug level: %s", r)
			}
		}
	}
	if debug > 0 && loud == 0 {
		ok("sshd logs go to the kernel log at debug level (%d lines), not the console", debug)
	} else if debug == 0 {
		bad("no sshd lines in the kernel log")
	}
}

// runChecks runs the banner loop the way init does, into a file instead of
// tty0, with a FATAL line in the kernel log. Each banner block shows it once.
func runChecks() {
	if k, err := os.OpenFile("/dev/kmsg", os.O_WRONLY, 0); err == nil {
		fmt.Fprintf(k, "<2>identity: FATAL agentcheck test line\n")
		k.Close()
	}
	out := "/tmp/tty.txt"
	must(os.WriteFile(out, nil, 0o644))
	cmd := exec.Command(agent, "run", "-tty", out, "-heartbeat", "1s", "-efivars", "/tmp/no-efivars")
	cmd.Env = withoutVerbose()
	must(cmd.Start())
	time.Sleep(7 * time.Second)
	cmd.Process.Kill()
	cmd.Wait()
	b, _ := os.ReadFile(out)
	t := string(b)
	blocks := strings.Count(t, "· devlayer node")
	fatals := strings.Count(t, "identity: FATAL agentcheck test line")
	switch {
	case blocks == 0:
		bad("run: no banner on the tty\n%s", t)
	case fatals != blocks:
		bad("run: %d banner(s) but the FATAL line %d times\n%s", blocks, fatals, t)
	case strings.Contains(t, "node: "):
		bad("run: a heartbeat line on the tty (it scrolls the banner away)\n%s", t)
	case !strings.Contains(t, "\x1b[H") || strings.Count(t, "\x1b[2J") != 1:
		bad("run: the banner is not painted in place (cursor home, one clear)\n%q", t)
	case !kmsgHas("node: "):
		bad("run: no heartbeat line in the kernel log")
	default:
		ok("run: %d banner(s) in place, the FATAL line once in each, heartbeat in kmsg only", blocks)
	}

	// node_verbose=1: the classic scrolling banner, heartbeat lines on the tty.
	must(os.WriteFile(out, nil, 0o644))
	cmd = exec.Command(agent, "run", "-tty", out, "-heartbeat", "1s", "-efivars", "/tmp/no-efivars", "-settle", "0s")
	cmd.Env = append(withoutVerbose(), "node_verbose=1")
	must(cmd.Start())
	time.Sleep(4 * time.Second)
	cmd.Process.Kill()
	cmd.Wait()
	b, _ = os.ReadFile(out)
	if t := string(b); !strings.Contains(t, "node: ") || strings.Contains(t, "\x1b[2J") {
		bad("run node_verbose=1: want scrolling heartbeat lines and no clear\n%q", t)
	} else {
		ok("run node_verbose=1: classic scrolling banner and heartbeat")
	}

	// The shim line and the EFI report, from a fake efivarfs.
	dir := "/tmp/efivars-shim"
	os.MkdirAll(dir, 0o755)
	put := func(name string, b []byte) { must(os.WriteFile(dir+"/"+name, append([]byte{7, 0, 0, 0}, b...), 0o644)) }
	put("PennerShimBuild-c6f2a1d4-5b3e-4f8a-9d71-2e6b0c4a8f15", []byte("build=3 role=promoted bootcurrent=0002 path=\\EFI\\penner\\shim-next.efi"))
	upd := make([]byte, 41)
	upd[0], upd[1], upd[33] = 3, 0xf8, 2
	put("ShimUpdate-c6f2a1d4-5b3e-4f8a-9d71-2e6b0c4a8f15", upd)
	put("BootOrder-8be4df61-93ca-11d2-aa0d-00e098032b8c", []byte{1, 0, 2, 0})
	put("Boot0001-8be4df61-93ca-11d2-aa0d-00e098032b8c", append([]byte{1, 0, 0, 0, 0, 0}, 'p', 0, 'e', 0, 'n', 0, 0, 0))
	b, _ = exec.Command(agent, "banner", "-efivars", dir).CombinedOutput()
	e, _ := exec.Command(agent, "efi", "-efivars", dir).CombinedOutput()
	for _, want := range []string{" shim     build=3 role=promoted bootcurrent=0002", "ShimUpdate Failed build 2 f80000"} {
		if !strings.Contains(string(b), want) {
			bad("banner lacks %q\n%s", want, b)
		}
	}
	for _, want := range []string{"efi: BootOrder 0001,0002", `efi: Boot0001 "pen"`} {
		if !strings.Contains(string(e), want) {
			bad("efi report lacks %q\n%s", want, e)
		}
	}
	ok("shim line and EFI report from efivarfs")
}

// withoutVerbose is the environment minus node_verbose (vmrun sets it).
func withoutVerbose() []string {
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "node_verbose=") {
			env = append(env, e)
		}
	}
	return env
}

// kmsgHas: some kernel log record contains s.
func kmsgHas(s string) bool {
	fd, err := syscall.Open("/dev/kmsg", syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return false
	}
	defer syscall.Close(fd)
	buf := make([]byte, 8192)
	for {
		n, err := syscall.Read(fd, buf)
		if err == syscall.EPIPE {
			continue
		}
		if err != nil || n <= 0 {
			return false
		}
		if strings.Contains(string(buf[:n]), s) {
			return true
		}
	}
}

func main() {
	mode := os.Getenv("node_check")
	if mode == "" {
		mode = "watch"
	}
	fmt.Println("mode:", mode)
	s := startServer()
	v1, v2, v3, v4 := "20261017T000001Z", "20261017T000002Z", "20261017T000003Z", "20261017T000004Z"

	if mode == "dashboard" {
		// The dashboard on the real tty0 for a screendump (dashboard_vm),
		// named as the SER9 is: its FQDN in the header, the host row and ssh.
		must(syscall.Sethostname([]byte("ser9.penner.network")))
		dir := "/tmp/efivars-ui"
		os.MkdirAll(dir, 0o755)
		os.WriteFile(dir+"/PennerShimBuild-c6f2a1d4-5b3e-4f8a-9d71-2e6b0c4a8f15",
			append([]byte{6, 0, 0, 0}, "build=4 role=promoted bootcurrent=0002 path=\\EFI\\penner\\shim-next.efi"...), 0o644)
		s.publish(v1, false)
		cmd := exec.Command(agent, "run", "-tty", "/dev/tty0", "-efivars", dir, "-index", s.url, "-roots", "/tmp/ca.pem",
			"-interval", "1h", "-early", "0s")
		cmd.Env = append(withoutVerbose(), "node_release="+v1)
		must(cmd.Start())
		time.Sleep(8 * time.Second)
		fmt.Println("dashboard: painted")
		time.Sleep(6 * time.Second)
		cmd.Process.Kill()
		cmd.Wait()
		ok("dashboard painted on tty0")
		// The FQDN in the header, the host row with the address beside it,
		// and the ssh hint; never the bare domain.
		bb, _ := exec.Command(agent, "banner", "-efivars", dir).CombinedOutput()
		t := regexp.MustCompile("\x1b\\[[0-9;]*m").ReplaceAllString(string(bb), "")
		for _, want := range []string{"== ser9.penner.network · devlayer node", "host     ser9.penner.network  10.0.2.15", "ssh root@ser9.penner.network", "or root@10.0.2.15"} {
			if !strings.Contains(t, want) {
				bad("dashboard: no %q in\n%s", want, t)
			}
		}
		if strings.Contains(t, "· penner.network") {
			bad("dashboard: the bare domain in\n%s", t)
		}
		ok("dashboard names the node ser9.penner.network")
		done()
	}

	if mode == "optout" {
		s.publish(v2, false)
		log, m := watch(s, v1, 3)
		expect("node_autoreboot=off", log, m, "", "auto-reboot off (node_autoreboot=off)")
		done()
	}

	bannerChecks()
	sshdLogChecks()
	runChecks()

	s.publish(v1, false)
	log, m := watch(s, v1, 3)
	expect("same release", log, m, "", "up to date: "+v1)

	// The first good answer blesses the release: ShimTrial goes away, so the
	// shim does not roll back after boot_tries boots. Not again once gone.
	os.MkdirAll("/tmp/efivars", 0o755)
	trial := "/tmp/efivars/ShimTrial-c6f2a1d4-5b3e-4f8a-9d71-2e6b0c4a8f15"
	os.WriteFile(trial, []byte{7, 0, 0, 0, 1, 2}, 0o644)
	log, m = watch(s, v1, 1, "-efivars", "/tmp/efivars")
	expect("bless", log, m, "", "blessed this release for the shim (deleted ShimTrial)")
	if _, err := os.Stat(trial); err == nil {
		bad("bless: ShimTrial is still there")
	}
	log, m = watch(s, v1, 1, "-efivars", "/tmp/efivars")
	if strings.Contains(log, "blessed") {
		bad("bless: blessed again with no ShimTrial\n%s", log)
	}
	// A failed fetch is no proof of health: no bless.
	os.WriteFile(trial, []byte{7, 0, 0, 0, 1, 2}, 0o644)
	s.failIdx = true
	log, m = watch(s, v1, 1, "-efivars", "/tmp/efivars")
	s.failIdx = false
	if _, err := os.Stat(trial); err != nil || strings.Contains(log, "blessed") {
		bad("bless: blessed after a failed check\n%s", log)
	} else {
		ok("bless: not after a failed check")
	}
	os.Remove(trial)
	if s.notMod > 0 {
		ok("index re-checks are conditional (If-None-Match, %d x 304)", s.notMod)
	} else {
		bad("no 304: the agent did not send If-None-Match")
	}

	s.failIdx = true
	log, m = watch(s, v1, 2)
	expect("index HTTP 500", log, m, "", "release check failed, retrying in 200ms; not rebooting")
	s.failIdx = false

	log, m = watch(s, v1, 2, "-index", "https://127.0.0.1:1/index.json")
	expect("index unreachable", log, m, "", "release check failed, retrying in 200ms; not rebooting")

	// Right after boot a failure is "waiting for the network", retried fast,
	// and the first good answer ends that.
	s.failIdx = true
	log, m = watch(s, v1, 2, "-early", "1h", "-early-retry", "100ms")
	expect("index fails right after boot", log, m, "", "waiting for the network, retrying every 100ms")
	s.failIdx = false

	s.publish(v3, true)
	log, m = watch(s, v1, 2)
	expect("corrupt initrd", log, m, "", "sha256 mismatch")

	d2 := s.publish(v2, false)
	log, m = watch(s, v1, 3)
	expect("new release", log, m, d2, "NEW RELEASE "+v2)
	if r := readRecord(); r.Target == d2 && r.Tries == 1 {
		ok("reboot record: %s tries 1", d2[:19])
	} else {
		bad("reboot record %+v", r)
	}

	// The shim booted the old release again: the node is still on v1.
	log, m = watch(s, v1, 2, "-retry-base", "1h")
	expect("loop guard", log, m, "", "next try at uptime 1h0m0s")
	log, m = watch(s, v1, 2, "-retry-base", "1ms")
	expect("loop guard after the retry wait", log, m, d2, "NEW RELEASE "+v2)
	if r := readRecord(); r.Target == d2 && r.Tries == 2 {
		ok("reboot record: tries 2")
	} else {
		bad("reboot record %+v", r)
	}

	d4 := s.publish(v4, false)
	log, m = watch(s, v1, 3, "-retry-base", "1h")
	expect("a newer target resets the count", log, m, d4, "NEW RELEASE "+v4)
	if r := readRecord(); r.Target == d4 && r.Tries == 1 {
		ok("reboot record: %s tries 1", d4[:19])
	} else {
		bad("reboot record %+v", r)
	}

	// Booted into v4: nothing to do.
	log, m = watch(s, v4, 2)
	expect("after the reboot", log, m, "", "up to date: "+v4)
	done()
}

func done() {
	if failed > 0 {
		fmt.Printf("%d check(s) failed\n", failed)
		os.Exit(1)
	}
	fmt.Println("all checks passed")
	os.Exit(0)
}
