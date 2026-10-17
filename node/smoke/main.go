// smoke runs inside the node VM. It starts sysroot programs from the EROFS
// root through their devlayer wrappers, and checks that /etc and /tmp
// take writes on the tmpfs overlay.
package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	cmds := [][]string{
		{"/bin/sh", "-c", "echo sh ok"},
		{"/bin/git", "--version"},
		{"/bin/rg", "--version"},
		{"/bin/nvim", "--headless", "--version"},
		{"/bin/htop", "--version"},
		{"/bin/jq", "--version"},
		{"/bin/zig", "version"},
		{"/bin/go", "version"},
	}
	failed := 0
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Env = []string{"PATH=/bin", "HOME=/tmp", "TERM=dumb", "GOCACHE=/tmp/gocache"}
		out, err := cmd.CombinedOutput()
		first := string(out)
		for i, r := range first {
			if r == '\n' {
				first = first[:i]
				break
			}
		}
		if err != nil {
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
	if failed > 0 {
		os.Exit(1)
	}
}
