package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/stefanpenner/devlayer/internal/ssh"
)

const statusScript = `
    _dp="${DEVLAYER_PREFIX:-$HOME/.local}"
    export PATH="$_dp/bin:$_dp/git/bin:$_dp/zsh/bin:$_dp/go/bin:$PATH"
    for cmd in zsh git nvim go zig make fzf fd bat rg eza delta jq direnv lazygit gh htop devlayer; do
      if command -v "$cmd" > /dev/null 2>&1; then
        case "$cmd" in
          zsh)     ver=$(zsh --version 2>&1) ;;
          git)     ver=$(git --version 2>&1) ;;
          nvim)    ver=$(nvim --version 2>&1 | head -1) ;;
          go)      ver=$(go version 2>&1) ;;
          htop)    ver=$(htop --version 2>&1 | head -1) ;;
          zig)     ver=$(zig version 2>&1) ;;
          make)    ver=$(make --version 2>&1 | head -1) ;;
          lazygit) ver=$(lazygit --version 2>&1 | head -1) ;;
          devlayer) ver=$(devlayer version 2>&1) ;;
          *)       ver=$("$cmd" --version 2>&1 | head -1) ;;
        esac
        printf "  %-10s %s\n" "$cmd" "$ver"
      else
        printf "  %-10s (not installed)\n" "$cmd"
      fi
    done
`

// Status shows installed tool versions on a host, or locally if no host given.
func Status(host string) error {
	if host == "" {
		fmt.Println("==> Versions (local):")
		if runtime.GOOS == "windows" {
			return statusNative()
		}
		cmd := exec.Command("sh", "-c", statusScript)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	fmt.Printf("==> Versions on %s:\n", host)
	return ssh.RunInteractive(host, statusScript)
}

// statusNative checks tool versions using Go exec (no shell required).
func statusNative() error {
	tools := []struct {
		name string
		args []string
	}{
		{"nvim", []string{"--version"}},
		{"go", []string{"version"}},
		{"zig", []string{"version"}},
		{"make", []string{"--version"}},
		{"fzf", []string{"--version"}},
		{"fd", []string{"--version"}},
		{"bat", []string{"--version"}},
		{"rg", []string{"--version"}},
		{"eza", []string{"--version"}},
		{"delta", []string{"--version"}},
		{"jq", []string{"--version"}},
		{"direnv", []string{"--version"}},
		{"lazygit", []string{"--version"}},
		{"gh", []string{"--version"}},
		{"devlayer", []string{"version"}},
	}

	for _, t := range tools {
		if _, err := exec.LookPath(t.name); err != nil {
			fmt.Printf("  %-10s (not installed)\n", t.name)
			continue
		}
		out, err := exec.Command(t.name, t.args...).CombinedOutput()
		if err != nil {
			fmt.Printf("  %-10s (error)\n", t.name)
			continue
		}
		ver := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
		fmt.Printf("  %-10s %s\n", t.name, ver)
	}
	return nil
}
