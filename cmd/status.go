package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/stefanpenner/devlayer/internal/ssh"
)

// statusTool is one probe in the version catalog.
// full: shell prints every line. Windows still prints the first.
// shellOnly: not probed on Windows.
type statusTool struct {
	name      string
	arg       string
	full      bool
	shellOnly bool
}

// statusCatalog is the version list for local status and SSH.
var statusCatalog = []statusTool{
	{name: "zsh", arg: "--version", full: true, shellOnly: true},
	{name: "git", arg: "--version", full: true, shellOnly: true},
	{name: "nvim", arg: "--version"},
	{name: "go", arg: "version", full: true},
	{name: "zig", arg: "version", full: true},
	{name: "make", arg: "--version"},
	{name: "fzf", arg: "--version"},
	{name: "fd", arg: "--version"},
	{name: "bat", arg: "--version"},
	{name: "rg", arg: "--version"},
	{name: "eza", arg: "--version"},
	{name: "delta", arg: "--version"},
	{name: "jq", arg: "--version"},
	{name: "direnv", arg: "--version"},
	{name: "lazygit", arg: "--version"},
	{name: "gh", arg: "--version"},
	{name: "htop", arg: "--version", shellOnly: true},
	{name: "devlayer", arg: "version", full: true},
}

// Status shows installed tool versions on a host, or locally if no host given.
func Status(host string) error {
	if host != "" {
		fmt.Printf("==> Versions on %s:\n", host)
		return ssh.RunInteractive(host, renderStatusShell())
	}

	fmt.Println("==> Versions (local):")
	if runtime.GOOS == "windows" {
		return statusWindows()
	}
	return statusUnix()
}

func statusUnix() error {
	cmd := exec.Command("sh", "-c", renderStatusShell())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func statusWindows() error {
	for _, t := range statusCatalog {
		if t.shellOnly {
			continue
		}
		fmt.Printf("  %-10s %s\n", t.name, windowsDetail(t))
	}
	return nil
}

func windowsDetail(t statusTool) string {
	if _, err := exec.LookPath(t.name); err != nil {
		return "(not installed)"
	}
	out, err := exec.Command(t.name, t.arg).CombinedOutput()
	if err != nil {
		return "(error)"
	}
	return strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
}

func renderStatusShell() string {
	var b strings.Builder
	b.WriteString(`
    _dp="${DEVLAYER_PREFIX:-$HOME/.local}"
    export PATH="$_dp/bin:$_dp/git/bin:$_dp/zsh/bin:$_dp/go/bin:$PATH"
    for cmd in `)
	for i, t := range statusCatalog {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(t.name)
	}
	b.WriteString(`; do
      if command -v "$cmd" > /dev/null 2>&1; then
        case "$cmd" in
`)
	for _, t := range statusCatalog {
		fmt.Fprintf(&b, "          %s) ver=%s ;;\n", t.name, shellCapture(t))
	}
	b.WriteString(`        esac
        printf "  %-10s %s\n" "$cmd" "$ver"
      else
        printf "  %-10s (not installed)\n" "$cmd"
      fi
    done
`)
	return b.String()
}

func shellCapture(t statusTool) string {
	cmd := t.name + " " + t.arg
	if t.full {
		return "$(" + cmd + " 2>&1)"
	}
	return "$(" + cmd + " 2>&1 | head -1)"
}
