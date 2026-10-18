// smoke runs inside the node VM. The node boots from the runtime layer; the
// test attaches the dev layer (the full devlayer sysroot) at /layers/dev.
// smoke starts sysroot programs from that layer through their devlayer
// wrappers, and checks that /etc and /tmp take writes on the tmpfs overlay.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	const dev = "/layers/dev/bin/"
	failed := 0
	target, err := os.Readlink("/layers/dev")
	if err != nil || !strings.HasPrefix(target, ".by-hash/") {
		failed++
		fmt.Printf("FAIL /layers/dev is not a content-hash link: %q %v\n", target, err)
	} else {
		fmt.Printf("ok   /layers/dev -> %s\n", target)
	}
	cmds := [][]string{
		{"/bin/sh", "-c", "echo runtime sh ok"},
		{dev + "git", "--version"},
		{dev + "rg", "--version"},
		{dev + "nvim", "--headless", "--version"},
		{dev + "htop", "--version"},
		{dev + "jq", "--version"},
		{dev + "zig", "version"},
		{dev + "go", "version"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Env = []string{"PATH=" + dev + ":/bin", "HOME=/tmp", "TERM=dumb", "GOCACHE=/tmp/gocache"}
		out, err := cmd.CombinedOutput()
		first := string(out)
		for i, r := range first {
			if r == '\n' {
				first = first[:i]
				break
			}
		}
		if err != nil || strings.Contains(string(out), "not found") {
			failed++
			fmt.Printf("FAIL %v: %v: %s\n", c, err, out)
			continue
		}
		fmt.Printf("ok   %s: %s\n", c[0], first)
	}
	for _, p := range []string{"/etc/node-smoke", "/tmp/node-smoke", "/var/node-smoke"} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			failed++
			fmt.Printf("FAIL write %s: %v\n", p, err)
			continue
		}
		fmt.Printf("ok   write %s\n", p)
	}
	if err := os.WriteFile("/bin/node-smoke", nil, 0o644); err != nil {
		failed++
		fmt.Printf("FAIL write /bin: %v\n", err)
	}
	if err := os.WriteFile(dev+"node-smoke", nil, 0o644); err == nil {
		failed++
		fmt.Println("FAIL the dev layer took a write")
	} else {
		fmt.Println("ok   the dev layer is read-only")
	}
	if failed > 0 {
		os.Exit(1)
	}
}
