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
		msg = strings.TrimRight(strings.SplitN(msg, "\n", 2)[0], " ")
		if strings.Contains(msg, "FATAL") || strings.Contains(msg, "identity: WARNING") {
			if len(k.lines) == 0 || k.lines[len(k.lines)-1] != msg {
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

// render draws the banner. withUptime=false gives the text that decides
// whether anything changed.
func render(in info, color bool) string {
	c := func(code, s string) string {
		if !color {
			return s
		}
		return code + s + reset
	}
	var b strings.Builder
	line := strings.Repeat("=", 72)
	fmt.Fprintf(&b, "\n%s\n", c(bold, "== devlayer node "+line[17:]))
	fmt.Fprintf(&b, " host     %s   up %s\n", or(in.host, "(none)"), in.up)
	fmt.Fprintf(&b, " network  %s %s  %s\n", or(in.iface, "(no interface)"), or(in.mac, "-"), in.speed)
	fmt.Fprintf(&b, "          %s via %s  dns %s\n", or(in.addr, "(no address)"), or(in.gw, "-"), or(in.dns, "-"))
	fmt.Fprintf(&b, " release  %s", or(in.release, "(no node_release on the command line)"))
	if in.latest != "" {
		fmt.Fprintf(&b, "   index: %s", in.latest)
	}
	b.WriteString("\n")
	tpm := c(red, "no (enable AMD fTPM in BIOS: Security)")
	if in.tpm {
		tpm = c(green, "yes")
	}
	fmt.Fprintf(&b, " tpm      %s\n", tpm)
	if in.fingerprint != "" {
		fmt.Fprintf(&b, " host key %s (%s)\n", in.fingerprint, or(in.keySource, "?"))
	} else {
		fmt.Fprintf(&b, " host key %s\n", c(red, "none"))
	}
	if in.sshd {
		fmt.Fprintf(&b, " sshd     %s\n", c(green, "listening on port 22"))
		fmt.Fprintf(&b, " login    %s\n", c(bold, "ssh root@"+or(in.ip, "<ip>")))
	} else {
		fmt.Fprintf(&b, " sshd     %s\n", c(red, "not running"))
	}
	if in.update != "" {
		u := in.update
		if in.updateAlert {
			u = c(yellow, u)
		}
		fmt.Fprintf(&b, " update   %s\n", u)
	}
	for _, f := range in.fatals {
		fmt.Fprintf(&b, " %s\n", c(red, f))
	}
	fmt.Fprintf(&b, "%s\n", c(bold, line))
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

func toTTY(path, s string) {
	f, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NOCTTY, 0)
	if err != nil {
		return
	}
	f.WriteString(strings.ReplaceAll(s, "\n", "\r\n"))
	f.Close()
}

func bannerLoop(c config, st *status) {
	k := openKmsg()
	last := ""
	lastBeat, lastFull := time.Now(), time.Now()
	for {
		in := collect(st, k)
		key := in
		key.up = 0
		if s := render(key, false); s != last || time.Since(lastFull) >= 10*time.Minute {
			last, lastFull = s, time.Now()
			toTTY(c.tty, render(in, true))
			logk(7, render(in, false))
			lastBeat = time.Now()
		} else if time.Since(lastBeat) >= c.heartbeat {
			toTTY(c.tty, heartbeat(in)+"\n")
			lastBeat = time.Now()
		}
		time.Sleep(2 * time.Second)
	}
}
