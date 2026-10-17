package main

// The status banner on tty0.

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	identityPub = "/run/node-identity/ssh_host_ecdsa_key.pub"
	red         = "\x1b[1;31m"
	green       = "\x1b[1;32m"
	yellow      = "\x1b[1;33m"
	bold        = "\x1b[1m"
	reset       = "\x1b[0m"
)

type info struct {
	host, iface, mac, addr, gw, dns, speed string
	release, latest                        string
	tpm                                    bool
	keySource, fingerprint                 string
	sshd                                   bool
	ip                                     string
	update                                 string
	updateAlert                            bool
	fatals                                 []string
	shim                                   string
	up                                     time.Duration
}

// kmsgLog keeps the FATAL and WARNING lines from the kernel log, which is
// where init and node-identity report them.
type kmsgLog struct {
	f     *os.File
	lines []string
}

func openKmsg() *kmsgLog {
	fd, err := syscall.Open("/dev/kmsg", syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return &kmsgLog{}
	}
	return &kmsgLog{f: os.NewFile(uintptr(fd), "/dev/kmsg")}
}

func (k *kmsgLog) poll() []string {
	if k.f == nil {
		return k.lines
	}
	buf := make([]byte, 8192)
	for {
		n, err := syscall.Read(int(k.f.Fd()), buf)
		if err == syscall.EPIPE {
			continue // records were overwritten; go on with the next
		}
		if err != nil || n <= 0 {
			break
		}
		rec := string(buf[:n])
		_, msg, ok := strings.Cut(rec, ";")
		if !ok {
			continue
		}
		msg = strings.TrimSpace(strings.SplitN(msg, "\n", 2)[0])
		// Skip node-agent's own lines: the banner mirror repeats the FATAL
		// lines it shows.
		if strings.HasPrefix(msg, "node-agent") {
			continue
		}
		if strings.Contains(msg, "FATAL") || strings.Contains(msg, "identity: WARNING") {
			seen := false
			for _, l := range k.lines {
				seen = seen || l == msg
			}
			if !seen {
				k.lines = append(k.lines, msg)
			}
			if len(k.lines) > 4 {
				k.lines = k.lines[len(k.lines)-4:]
			}
		}
	}
	return k.lines
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// defaultRoute reads /proc/net/route: the interface and gateway of 0.0.0.0/0.
func defaultRoute() (string, string) {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return "", ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		fl := strings.Fields(s.Text())
		if len(fl) < 3 || fl[1] != "00000000" {
			continue
		}
		b, err := hex.DecodeString(fl[2])
		if err != nil || len(b) != 4 {
			continue
		}
		// The kernel prints the network-order address as a host-order (little
		// endian) word, so the bytes come out reversed.
		return fl[0], net.IPv4(b[3], b[2], b[1], b[0]).String()
	}
	return "", ""
}

// sshdListening looks for a listening socket on port 22 in /proc/net/tcp{,6}.
func sshdListening() bool {
	for _, p := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, l := range strings.Split(string(b), "\n")[1:] {
			fl := strings.Fields(l)
			if len(fl) > 3 && strings.HasSuffix(fl[1], ":0016") && fl[3] == "0A" {
				return true
			}
		}
	}
	return false
}

func collect(st *status, k *kmsgLog) info {
	var in info
	in.host, _ = os.Hostname()
	in.iface, in.gw = defaultRoute()
	if in.iface == "" {
		if ifs, err := net.Interfaces(); err == nil {
			for _, i := range ifs {
				if i.Flags&net.FlagLoopback == 0 {
					in.iface = i.Name
					break
				}
			}
		}
	}
	if i, err := net.InterfaceByName(in.iface); err == nil {
		in.mac = i.HardwareAddr.String()
		if addrs, err := i.Addrs(); err == nil {
			for _, a := range addrs {
				if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
					in.addr = n.String()
					in.ip = n.IP.String()
					break
				}
			}
		}
	}
	if sp, err := strconv.Atoi(readTrim("/sys/class/net/" + in.iface + "/speed")); err == nil && sp > 0 {
		if sp >= 1000 {
			in.speed = fmt.Sprintf("%g Gb/s", float64(sp)/1000)
		} else {
			in.speed = fmt.Sprintf("%d Mb/s", sp)
		}
	} else if readTrim("/sys/class/net/"+in.iface+"/operstate") == "up" {
		in.speed = "link up"
	} else {
		in.speed = "no link"
	}
	for _, l := range strings.Split(readTrim("/etc/resolv.conf"), "\n") {
		if v, ok := strings.CutPrefix(l, "nameserver "); ok {
			in.dns = strings.TrimSpace(v)
			break
		}
	}
	in.release = os.Getenv("node_release")
	st.mu.Lock()
	in.latest = st.latest
	st.mu.Unlock()
	in.update, in.updateAlert = st.get()
	_, err := os.Stat("/dev/tpmrm0")
	in.tpm = err == nil
	in.keySource = readTrim("/run/node-identity/source")
	if b, err := os.ReadFile(identityPub); err == nil {
		if pk, _, _, _, err := ssh.ParseAuthorizedKey(b); err == nil {
			in.fingerprint = ssh.FingerprintSHA256(pk)
		}
	}
	in.sshd = sshdListening()
	if k == nil {
		k = openKmsg()
		defer func() {
			if k.f != nil {
				k.f.Close()
			}
		}()
	}
	in.fatals = append([]string(nil), k.poll()...)
	in.up = uptime().Truncate(time.Second)
	return in
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// The dashboard: Tokyo Night colors (init loads the palette into the VT,
// so these ANSI colors come out as the Tokyo Night values), the devlayer
// wordmark, aligned sections, errors in red, and a footer.
const (
	blue  = "\x1b[1;34m" // #7aa2f7
	cyan  = "\x1b[1;36m" // #7dcfff
	gray  = "\x1b[1;30m" // #565f89
	white = "\x1b[1;37m" // #c0caf5
)

var logo = []string{
	`     _            _                       `,
	`  __| | _____   _| | __ _ _   _  ___ _ __ `,
	` / _` + "`" + ` |/ _ \ \ / / |/ _` + "`" + ` | | | |/ _ \ '__|`,
	`| (_| |  __/\ V /| | (_| | |_| |  __/ |   `,
	` \__,_|\___| \_/ |_|\__,_|\__, |\___|_|   `,
	`                          |___/           `,
}

const width = 72

// render draws the dashboard. color=false gives plain text: the kernel log
// copy, `node-agent banner`, and the text that decides whether anything
// changed (with in.up zeroed).
func render(in info, color bool) string {
	c := func(code, s string) string {
		if !color {
			return s
		}
		return code + s + reset
	}
	// Not up yet in the first 30 s: "starting" in yellow, not an error.
	early := in.up < 30*time.Second
	bad := func(starting, failed string) string {
		if early {
			return c(yellow, starting)
		}
		return c(red, failed)
	}
	var b strings.Builder
	row := func(label, value string) { fmt.Fprintf(&b, "   %s %s\n", c(gray, fmt.Sprintf("%-8s", label)), value) }
	section := func(name string) { fmt.Fprintf(&b, "\n %s\n", c(blue, name)) }

	b.WriteString("\n")
	for _, l := range logo {
		fmt.Fprintf(&b, " %s\n", c(blue, l))
	}
	fmt.Fprintf(&b, " %s %s\n", c(cyan, "== devlayer node"), c(gray, "· penner.network"))

	section("IDENTITY")
	row("host", c(white, or(in.host, "(none)")))
	if in.tpm {
		row("tpm", c(green, "yes"))
	} else {
		row("tpm", c(red, "no (enable AMD fTPM in BIOS: Security)"))
	}
	if in.fingerprint != "" {
		row("host key", fmt.Sprintf("%s (%s)", in.fingerprint, or(in.keySource, "?")))
	} else {
		row("host key", bad("starting…", "none"))
	}

	section("NETWORK")
	row("link", fmt.Sprintf("%s %s  %s", or(in.iface, "(no interface)"), or(in.mac, "-"), in.speed))
	row("address", fmt.Sprintf("%s via %s  dns %s", or(in.addr, "(no address)"), or(in.gw, "-"), or(in.dns, "-")))

	section("RELEASE")
	row("running", c(white, or(in.release, "(no node_release on the command line)")))
	if in.latest != "" {
		row("index", in.latest)
	}
	if in.shim != "" {
		row("shim", in.shim)
	}

	section("SERVICES")
	if in.sshd {
		row("sshd", c(green, "listening on port 22"))
	} else {
		row("sshd", bad("starting…", "not running"))
	}
	if in.update != "" {
		u := in.update
		if in.updateAlert {
			u = c(yellow, u)
		}
		row("update", u)
	}

	if len(in.fatals) > 0 {
		section(c(red, "ERRORS"))
		for _, f := range in.fatals {
			fmt.Fprintf(&b, "   %s\n", c(red, f))
		}
	}

	left := "up " + in.up.String()
	right := "ssh root@" + or(in.ip, "<ip>")
	if !in.sshd {
		right = "ssh: not yet"
	}
	pad := width - 2 - len(left) - len(right)
	if pad < 2 {
		pad = 2
	}
	fmt.Fprintf(&b, "\n %s\n", c(gray, strings.Repeat("─", width-1)))
	fmt.Fprintf(&b, " %s%s%s\n", c(gray, left), strings.Repeat(" ", pad), c(bold, right))
	return b.String()
}

func heartbeat(in info) string {
	ssh := "sshd down"
	if in.sshd {
		ssh = "ssh root@" + in.ip
	}
	return fmt.Sprintf("node: %s | %s | %s | release %s | up %s | %s",
		or(in.host, "(none)"), or(in.addr, "no address"), ssh, or(in.release, "?"), in.up, or(in.update, "-"))
}

// repaint draws s over the previous banner: cursor home, each line cleared
// to its end, then the rest of the screen cleared. No scrolling, so a
// console line in between is painted over at the next repaint.
func repaint(s string, first bool) string {
	var b strings.Builder
	if first {
		b.WriteString("\x1b[?25l\x1b[H\x1b[2J") // hide the cursor, clear once
	}
	b.WriteString("\x1b[H")
	for _, l := range strings.Split(strings.TrimPrefix(s, "\n"), "\n") {
		b.WriteString(l + "\x1b[K\n")
	}
	b.WriteString("\x1b[J")
	return b.String()
}

func toTTY(path, s string) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|syscall.O_NOCTTY, 0)
	if err != nil {
		return
	}
	f.WriteString(strings.ReplaceAll(s, "\n", "\r\n"))
	f.Close()
}

// bannerLoop keeps the dashboard in place on tty0: a repaint every 2 s when
// anything changed, and every 10 s for the uptime. The kernel log gets the
// whole block when it changes, and the one-line heartbeat every minute;
// both at debug level, so they never scroll the monitor.
//
// The first paint waits up to 3 s for the host key and sshd, so a normal
// boot never flashes "starting". node_verbose=1 gives the classic
// scrolling banner: printed on a change, and a heartbeat line every minute.
func bannerLoop(c config, st *status) {
	k := openKmsg()
	last := ""
	shim := shimSummary(c.efivars)
	for _, l := range efiReport(c.efivars) {
		logk(6, "node-agent: "+l)
	}
	verbose := os.Getenv("node_verbose") == "1"
	for deadline := time.Now().Add(c.settle); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if _, err := os.Stat(identityPub); err == nil && sshdListening() {
			break
		}
	}
	first := true
	lastBeat, lastPaint := time.Now(), time.Time{}
	for {
		in := collect(st, k)
		in.shim = shim
		key := in
		key.up = 0
		s := render(key, false)
		switch {
		case verbose && s != last:
			toTTY(c.tty, render(in, true))
		case !verbose && (s != last || time.Since(lastPaint) >= 10*time.Second):
			toTTY(c.tty, repaint(render(in, true), first))
			first, lastPaint = false, time.Now()
		}
		if s != last {
			last = s
			logk(7, "node-agent banner:"+strings.ReplaceAll(render(in, false), "\n", "\nnode-agent| "))
		}
		if time.Since(lastBeat) >= c.heartbeat {
			if verbose {
				toTTY(c.tty, heartbeat(in)+"\n")
			}
			logk(7, heartbeat(in))
			lastBeat = time.Now()
		}
		time.Sleep(2 * time.Second)
	}
}
