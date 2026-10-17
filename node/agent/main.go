// node-agent runs on every node, started by init after sshd. It does two
// things:
//
//   - The status banner. A block on the monitor (tty0) with the node's
//     address, link speed, release, TPM host key, sshd state, the exact ssh
//     command, uptime, and any FATAL line in red. It is printed again when
//     anything in it changes, in place (cursor home, no scrolling), and its
//     uptime every 10 s. The block, and a one-line heartbeat every minute,
//     go to the kernel log at debug level only, so dmesg has them and the
//     monitor never scrolls. A "shim" line says which efi-boot-shim booted
//     the node (PennerShimBuild), and dmesg gets the EFI boot variables.
//
//   - The release watcher. Every minute it fetches the netboot index
//     (https://boot.penner.network/index.json) with If-None-Match. When the
//     index names a release other than the one this node booted
//     (node_release=<version> on the kernel command line, added by the
//     publish script), it downloads the new manifest and every blob, checks
//     each sha256, announces the reboot, waits a grace period, and reboots.
//     The shim then boots the new release. See watch.go for the loop guard.
//
// Boot options (the kernel passes name=value options to init's environment):
//
//	node_release=<version>   the release this node booted; without it the
//	                         watcher is off
//	node_autoreboot=off      never reboot for a new release
//
// Usage:
//
//	node-agent [run]         banner and watcher, until killed
//	node-agent banner        print the banner once to stdout
//	node-agent record        print the reboot record from the TPM NV index
//	node-agent watch         the watcher alone; for tests, with -checks N and
//	                         -reboot-marker PATH instead of a reboot
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type config struct {
	index        string
	roots        string
	interval     time.Duration
	grace        time.Duration
	retryBase    time.Duration
	retryMax     time.Duration
	failWait     time.Duration
	early        time.Duration
	earlyRetry   time.Duration
	checks       int
	rebootMarker string
	tty          string
	heartbeat    time.Duration
	nvIndex      uint
	efivars      string
	release      string
	autoreboot   string
}

// status is what the watcher tells the banner.
type status struct {
	mu     sync.Mutex
	line   string
	alert  bool
	latest string
}

func (s *status) set(line string, alert bool) {
	s.mu.Lock()
	changed := s.line != line
	s.line, s.alert = line, alert
	s.mu.Unlock()
	if changed {
		level := 6
		if alert {
			level = 4
		}
		logk(level, "node-agent: "+line)
	}
}

func (s *status) get() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.line, s.alert
}

func main() {
	cmd := "run"
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var c config
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.StringVar(&c.index, "index", "https://boot.penner.network/index.json", "netboot OCI index URL")
	fs.StringVar(&c.roots, "roots", "", "PEM roots for the index server (default: the built-in ISRG Root YE and X2, as in the shim)")
	fs.DurationVar(&c.interval, "interval", 60*time.Second, "time between index checks")
	fs.DurationVar(&c.grace, "grace", 30*time.Second, "wait between announcing a reboot and rebooting")
	fs.DurationVar(&c.retryBase, "retry-base", 5*time.Minute, "wait after boot before retrying a release the shim did not boot; doubles per try")
	fs.DurationVar(&c.retryMax, "retry-max", 6*time.Hour, "longest retry wait")
	fs.DurationVar(&c.failWait, "fail-wait", 10*time.Minute, "wait after a release fails verification")
	fs.DurationVar(&c.early, "early", 3*time.Minute, "until this uptime, and until the first good check, a failed check retries after -early-retry")
	fs.DurationVar(&c.earlyRetry, "early-retry", 5*time.Second, "retry wait while the network comes up")
	fs.IntVar(&c.checks, "checks", 0, "exit after this many index checks (tests; 0 = forever)")
	fs.StringVar(&c.rebootMarker, "reboot-marker", "", "write the target here and exit instead of rebooting (tests)")
	fs.StringVar(&c.tty, "tty", "/dev/tty0", "where the banner goes")
	fs.DurationVar(&c.heartbeat, "heartbeat", 60*time.Second, "one-line status interval")
	fs.UintVar(&c.nvIndex, "nv-index", 0x01008001, "TPM NV index for the reboot record")
	fs.StringVar(&c.efivars, "efivars", defaultEfivars, "efivarfs directory with the shim's ShimTrial variable (empty: never bless)")
	fs.Parse(args)
	c.release = os.Getenv("node_release")
	c.autoreboot = os.Getenv("node_autoreboot")

	switch cmd {
	case "banner":
		var st status
		st.line = watcherOff(c)
		in := collect(&st, nil)
		in.shim = shimSummary(c.efivars)
		fmt.Print(render(in, false))
	case "efi":
		for _, l := range efiReport(c.efivars) {
			fmt.Println(l)
		}
	case "record":
		nv, err := openNV(uint32(c.nvIndex))
		if err != nil {
			fmt.Fprintln(os.Stderr, "node-agent:", err)
			os.Exit(1)
		}
		r, err := nv.read()
		nv.close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "node-agent:", err)
			os.Exit(1)
		}
		b, _ := json.Marshal(r)
		fmt.Println(string(b))
	case "watch":
		var st status
		os.Exit(watch(c, &st))
	case "run":
		toStderr = false
		var st status
		go func() {
			for {
				watch(c, &st)
				time.Sleep(c.interval)
			}
		}()
		bannerLoop(c, &st)
	default:
		fmt.Fprintln(os.Stderr, "usage: node-agent [run|banner|efi|record|watch] [flags]")
		os.Exit(2)
	}
}

// logk writes one line to stderr and to the kernel log at a syslog level.
// Levels 0-6 also print on the consoles; 7 (debug) is dmesg only.
// In run mode stderr is the console, so only the kernel log gets the line.
var toStderr = true

func logk(level int, msg string) {
	if toStderr {
		fmt.Fprintln(os.Stderr, msg)
	}
	if f, err := os.OpenFile("/dev/kmsg", os.O_WRONLY, 0); err == nil {
		for _, l := range strings.Split(strings.TrimRight(msg, "\n"), "\n") {
			fmt.Fprintf(f, "<%d>%s\n", level, l)
		}
		f.Close()
	}
}
