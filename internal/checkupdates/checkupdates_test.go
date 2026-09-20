package checkupdates

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stefanpenner/devlayer/internal/versions"
)

const sampleEnv = `# Pinned tool versions
FZF_VERSION=0.70.0
FD_VERSION=10.5.0
BAT_VERSION=0.26.1
EZA_VERSION=0.23.5
RG_VERSION=15.2.0
DELTA_VERSION=0.19.2
LAZYGIT_VERSION=0.65.1
BAT_EXTRAS_VERSION=2024.08.24
JQ_VERSION=1.8.2
DIRENV_VERSION=2.37.1
NVIM_VERSION=0.11.6
GO_VERSION=1.26.1
GIT_VERSION=2.53.0
GIT_WINDOWS_VERSION=2.55.0.5
ZSH_VERSION=5.9
HTOP_VERSION=3.4.1
BTOP_VERSION=1.4.7
DUST_VERSION=1.2.6
AGE_VERSION=1.3.2
ZIG_VERSION=0.15.2
MAKE_VERSION=4.4.1
NCURSES_VERSION=6.5

# Zsh plugins (shell scripts — portable across all platforms)
ZSH_AUTOSUGGESTIONS_VERSION=v0.7.1
FAST_SYNTAX_HIGHLIGHTING_VERSION=v1.56
`

func writeRunFiles(t *testing.T) (envPath, updates, ghOut string) {
	t.Helper()
	dir := t.TempDir()
	envPath = filepath.Join(dir, "versions.env")
	updates = filepath.Join(dir, "updates.txt")
	ghOut = filepath.Join(dir, "github_output")
	if err := os.WriteFile(envPath, []byte(sampleEnv), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ghOut, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return envPath, updates, ghOut
}

func getenvFor(updates, ghOut string) func(string) string {
	return func(k string) string {
		switch k {
		case "UPDATES_FILE":
			return updates
		case "GITHUB_OUTPUT":
			return ghOut
		default:
			return ""
		}
	}
}

func TestRunBumpsFZFLeavesBAT(t *testing.T) {
	envPath, updates, ghOut := writeRunFiles(t)
	get := func(url string) ([]byte, error) {
		switch url {
		case "https://api.github.com/repos/junegunn/fzf/releases/latest":
			return []byte(`{"tag_name":"v0.74.4"}`), nil
		case "https://api.github.com/repos/sharkdp/bat/releases/latest":
			return []byte(`{"tag_name":"v0.26.1"}`), nil
		default:
			return nil, fmt.Errorf("no latest")
		}
	}

	var stdout bytes.Buffer
	report, err := Run(envPath, get, false, getenvFor(updates, ghOut), &stdout)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Any() {
		t.Fatal("expected bumps")
	}
	if len(report.Bumps) != 1 || report.Bumps[0].Key != "FZF" {
		t.Fatalf("bumps = %+v, want only FZF", report.Bumps)
	}
	if report.Bumps[0].Current != "0.70.0" || report.Bumps[0].Latest != "0.74.4" {
		t.Fatalf("FZF bump = %+v", report.Bumps[0])
	}

	got, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	pins := versions.Parse(string(got))
	if pins.Get("FZF_VERSION") != "0.74.4" {
		t.Errorf("FZF_VERSION = %s, want 0.74.4", pins.Get("FZF_VERSION"))
	}
	if pins.Get("BAT_VERSION") != "0.26.1" {
		t.Errorf("BAT_VERSION = %s, want 0.26.1", pins.Get("BAT_VERSION"))
	}
	if pins.Get("ZSH_AUTOSUGGESTIONS_VERSION") != "v0.7.1" {
		t.Errorf("plugin key was rewritten")
	}
	if !strings.HasPrefix(string(got), "# Pinned tool versions\n") {
		t.Error("lost header comment")
	}

	up, err := os.ReadFile(updates)
	if err != nil {
		t.Fatal(err)
	}
	if string(up) != "FZF: 0.70.0 → 0.74.4\n" {
		t.Errorf("UPDATES_FILE = %q", up)
	}

	out, err := os.ReadFile(ghOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "keep\nhas_updates=true\n" {
		t.Errorf("GITHUB_OUTPUT = %q", out)
	}

	printed := stdout.String()
	if !strings.Contains(printed, "==> Updates found:") {
		t.Errorf("stdout = %q", printed)
	}
	if !strings.Contains(printed, "FZF: 0.70.0 → 0.74.4") {
		t.Errorf("stdout missing bump: %q", printed)
	}
}

func TestRunDryRunDoesNotWriteEnv(t *testing.T) {
	envPath, updates, ghOut := writeRunFiles(t)
	get := func(url string) ([]byte, error) {
		if url == "https://api.github.com/repos/junegunn/fzf/releases/latest" {
			return []byte(`{"tag_name":"v0.74.4"}`), nil
		}
		return nil, fmt.Errorf("no latest")
	}

	var stdout bytes.Buffer
	report, err := Run(envPath, get, true, getenvFor(updates, ghOut), &stdout)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Any() {
		t.Fatal("dry-run should still report bumps")
	}

	got, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if versions.Parse(string(got)).Get("FZF_VERSION") != "0.70.0" {
		t.Error("dry-run rewrote versions.env")
	}
}

func TestRunIgnoresPrerelease(t *testing.T) {
	envPath, updates, ghOut := writeRunFiles(t)
	get := func(url string) ([]byte, error) {
		if url == "https://api.github.com/repos/junegunn/fzf/releases/latest" {
			return []byte(`{"tag_name":"v0.80.0-rc1"}`), nil
		}
		return nil, fmt.Errorf("no latest")
	}

	var stdout bytes.Buffer
	report, err := Run(envPath, get, false, getenvFor(updates, ghOut), &stdout)
	if err != nil {
		t.Fatal(err)
	}
	if report.Any() {
		t.Fatalf("prerelease bumped: %+v", report.Bumps)
	}
	if !strings.Contains(stdout.String(), "==> All tools are up to date") {
		t.Errorf("stdout = %q", stdout.String())
	}

	up, err := os.ReadFile(updates)
	if err != nil {
		t.Fatal(err)
	}
	if string(up) != "" {
		t.Errorf("UPDATES_FILE = %q, want empty", up)
	}
	out, err := os.ReadFile(ghOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "keep\nhas_updates=false\n" {
		t.Errorf("GITHUB_OUTPUT = %q", out)
	}
}

func TestRunFailedFetchNoBumpNoCrash(t *testing.T) {
	envPath, updates, ghOut := writeRunFiles(t)
	get := func(string) ([]byte, error) {
		return nil, fmt.Errorf("HTTP 404")
	}

	var stdout bytes.Buffer
	report, err := Run(envPath, get, false, getenvFor(updates, ghOut), &stdout)
	if err != nil {
		t.Fatalf("failed fetch should not crash: %v", err)
	}
	if report.Any() {
		t.Fatalf("bumped on failed fetch: %+v", report.Bumps)
	}
}
