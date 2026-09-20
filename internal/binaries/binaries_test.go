package binaries

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stefanpenner/devlayer/internal/platform"
	"github.com/stefanpenner/devlayer/internal/versions"
)

const testEnv = `
FZF_VERSION=0.74.4
FD_VERSION=10.5.0
BAT_VERSION=0.26.1
EZA_VERSION=0.23.5
RG_VERSION=15.2.0
DELTA_VERSION=0.19.2
LAZYGIT_VERSION=0.65.1
BAT_EXTRAS_VERSION=2024.08.24
JQ_VERSION=1.8.2
DIRENV_VERSION=2.37.1
NVIM_VERSION=0.12.5
GO_VERSION=1.27.1
GIT_WINDOWS_VERSION=2.55.0.5
DUST_VERSION=1.2.6
AGE_VERSION=1.3.2
ZIG_VERSION=0.16.0
ZSH_AUTOSUGGESTIONS_VERSION=v0.7.1
FAST_SYNTAX_HIGHLIGHTING_VERSION=v1.56
ZSH_HISTORY_SUBSTRING_SEARCH_VERSION=v1.1.0
POWERLEVEL10K_VERSION=v1.20.0
`

func testVers() *versions.Versions {
	return versions.Parse(testEnv)
}

func mustURLs(t *testing.T, osName, arch string, vers *versions.Versions, skip map[string]bool) map[string]string {
	t.Helper()
	p, err := platform.New(osName, arch)
	if err != nil {
		t.Fatal(err)
	}
	got, err := URLs(p, vers, skip)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func eqURLs(t *testing.T, got, want map[string]string) {
	t.Helper()
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s\n  got  %q\n  want %q", k, got[k], v)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected %s = %q", k, got[k])
		}
	}
}

func TestURLs(t *testing.T) {
	vers := testVers()

	tests := []struct {
		name     string
		os, arch string
		skip     map[string]bool
		want     map[string]string
	}{
		{
			name: "linux/x86_64",
			os:   "linux", arch: "x86_64",
			want: map[string]string{
				"fd":                           "https://github.com/sharkdp/fd/releases/download/v10.5.0/fd-v10.5.0-x86_64-unknown-linux-musl.tar.gz",
				"bat":                          "https://github.com/sharkdp/bat/releases/download/v0.26.1/bat-v0.26.1-x86_64-unknown-linux-musl.tar.gz",
				"rg":                           "https://github.com/BurntSushi/ripgrep/releases/download/15.2.0/ripgrep-15.2.0-x86_64-unknown-linux-musl.tar.gz",
				"delta":                        "https://github.com/dandavison/delta/releases/download/0.19.2/delta-0.19.2-x86_64-unknown-linux-musl.tar.gz",
				"dust":                         "https://github.com/bootandy/dust/releases/download/v1.2.6/dust-v1.2.6-x86_64-unknown-linux-musl.tar.gz",
				"fzf":                          "https://github.com/junegunn/fzf/releases/download/v0.74.4/fzf-0.74.4-linux_amd64.tar.gz",
				"lazygit":                      "https://github.com/jesseduffield/lazygit/releases/download/v0.65.1/lazygit_0.65.1_Linux_x86_64.tar.gz",
				"age":                          "https://github.com/FiloSottile/age/releases/download/v1.3.2/age-v1.3.2-linux-amd64.tar.gz",
				"age-keygen":                   "https://github.com/FiloSottile/age/releases/download/v1.3.2/age-v1.3.2-linux-amd64.tar.gz",
				"direnv":                       "https://github.com/direnv/direnv/releases/download/v2.37.1/direnv.linux-amd64",
				"jq":                           "https://github.com/jqlang/jq/releases/download/jq-1.8.2/jq-linux-amd64",
				"batman":                       "https://github.com/eth-p/bat-extras/releases/download/v2024.08.24/bat-extras-2024.08.24.zip",
				"nvim":                         "https://github.com/neovim/neovim/releases/download/v0.12.5/nvim-linux-x86_64.tar.gz",
				"go":                           "https://go.dev/dl/go1.27.1.linux-amd64.tar.gz",
				"zig":                          "https://ziglang.org/download/0.16.0/zig-x86_64-linux-0.16.0.tar.xz",
				"fzf-shell":                    "https://github.com/junegunn/fzf/archive/refs/tags/v0.74.4.tar.gz",
				"zsh-autosuggestions":          "https://github.com/zsh-users/zsh-autosuggestions/archive/refs/tags/v0.7.1.tar.gz",
				"zsh-fast-syntax-highlighting": "https://github.com/zdharma-continuum/fast-syntax-highlighting/archive/refs/tags/v1.56.tar.gz",
				"zsh-history-substring-search": "https://github.com/zsh-users/zsh-history-substring-search/archive/refs/tags/v1.1.0.tar.gz",
				"powerlevel10k":                "https://github.com/romkatv/powerlevel10k/archive/refs/tags/v1.20.0.tar.gz",
			},
		},
		{
			name: "linux/aarch64",
			os:   "linux", arch: "aarch64",
			want: map[string]string{
				"fd":                           "https://github.com/sharkdp/fd/releases/download/v10.5.0/fd-v10.5.0-aarch64-unknown-linux-musl.tar.gz",
				"bat":                          "https://github.com/sharkdp/bat/releases/download/v0.26.1/bat-v0.26.1-aarch64-unknown-linux-musl.tar.gz",
				"rg":                           "https://github.com/BurntSushi/ripgrep/releases/download/15.2.0/ripgrep-15.2.0-aarch64-unknown-linux-musl.tar.gz",
				"delta":                        "https://github.com/dandavison/delta/releases/download/0.19.2/delta-0.19.2-aarch64-unknown-linux-gnu.tar.gz",
				"dust":                         "https://github.com/bootandy/dust/releases/download/v1.2.6/dust-v1.2.6-aarch64-unknown-linux-musl.tar.gz",
				"fzf":                          "https://github.com/junegunn/fzf/releases/download/v0.74.4/fzf-0.74.4-linux_arm64.tar.gz",
				"lazygit":                      "https://github.com/jesseduffield/lazygit/releases/download/v0.65.1/lazygit_0.65.1_Linux_arm64.tar.gz",
				"age":                          "https://github.com/FiloSottile/age/releases/download/v1.3.2/age-v1.3.2-linux-arm64.tar.gz",
				"age-keygen":                   "https://github.com/FiloSottile/age/releases/download/v1.3.2/age-v1.3.2-linux-arm64.tar.gz",
				"direnv":                       "https://github.com/direnv/direnv/releases/download/v2.37.1/direnv.linux-arm64",
				"jq":                           "https://github.com/jqlang/jq/releases/download/jq-1.8.2/jq-linux-arm64",
				"batman":                       "https://github.com/eth-p/bat-extras/releases/download/v2024.08.24/bat-extras-2024.08.24.zip",
				"nvim":                         "https://github.com/neovim/neovim/releases/download/v0.12.5/nvim-linux-arm64.tar.gz",
				"go":                           "https://go.dev/dl/go1.27.1.linux-arm64.tar.gz",
				"zig":                          "https://ziglang.org/download/0.16.0/zig-aarch64-linux-0.16.0.tar.xz",
				"fzf-shell":                    "https://github.com/junegunn/fzf/archive/refs/tags/v0.74.4.tar.gz",
				"zsh-autosuggestions":          "https://github.com/zsh-users/zsh-autosuggestions/archive/refs/tags/v0.7.1.tar.gz",
				"zsh-fast-syntax-highlighting": "https://github.com/zdharma-continuum/fast-syntax-highlighting/archive/refs/tags/v1.56.tar.gz",
				"zsh-history-substring-search": "https://github.com/zsh-users/zsh-history-substring-search/archive/refs/tags/v1.1.0.tar.gz",
				"powerlevel10k":                "https://github.com/romkatv/powerlevel10k/archive/refs/tags/v1.20.0.tar.gz",
			},
		},
		{
			name: "darwin/arm64",
			os:   "darwin", arch: "arm64",
			want: map[string]string{
				"fd":                           "https://github.com/sharkdp/fd/releases/download/v10.5.0/fd-v10.5.0-aarch64-apple-darwin.tar.gz",
				"bat":                          "https://github.com/sharkdp/bat/releases/download/v0.26.1/bat-v0.26.1-aarch64-apple-darwin.tar.gz",
				"rg":                           "https://github.com/BurntSushi/ripgrep/releases/download/15.2.0/ripgrep-15.2.0-aarch64-apple-darwin.tar.gz",
				"delta":                        "https://github.com/dandavison/delta/releases/download/0.19.2/delta-0.19.2-aarch64-apple-darwin.tar.gz",
				"dust":                         "https://github.com/bootandy/dust/releases/download/v1.2.6/dust-v1.2.6-aarch64-apple-darwin.tar.gz",
				"fzf":                          "https://github.com/junegunn/fzf/releases/download/v0.74.4/fzf-0.74.4-darwin_arm64.tar.gz",
				"lazygit":                      "https://github.com/jesseduffield/lazygit/releases/download/v0.65.1/lazygit_0.65.1_Darwin_arm64.tar.gz",
				"age":                          "https://github.com/FiloSottile/age/releases/download/v1.3.2/age-v1.3.2-darwin-arm64.tar.gz",
				"age-keygen":                   "https://github.com/FiloSottile/age/releases/download/v1.3.2/age-v1.3.2-darwin-arm64.tar.gz",
				"direnv":                       "https://github.com/direnv/direnv/releases/download/v2.37.1/direnv.darwin-arm64",
				"jq":                           "https://github.com/jqlang/jq/releases/download/jq-1.8.2/jq-macos-arm64",
				"batman":                       "https://github.com/eth-p/bat-extras/releases/download/v2024.08.24/bat-extras-2024.08.24.zip",
				"nvim":                         "https://github.com/neovim/neovim/releases/download/v0.12.5/nvim-macos-arm64.tar.gz",
				"go":                           "https://go.dev/dl/go1.27.1.darwin-arm64.tar.gz",
				"zig":                          "https://ziglang.org/download/0.16.0/zig-aarch64-macos-0.16.0.tar.xz",
				"fzf-shell":                    "https://github.com/junegunn/fzf/archive/refs/tags/v0.74.4.tar.gz",
				"zsh-autosuggestions":          "https://github.com/zsh-users/zsh-autosuggestions/archive/refs/tags/v0.7.1.tar.gz",
				"zsh-fast-syntax-highlighting": "https://github.com/zdharma-continuum/fast-syntax-highlighting/archive/refs/tags/v1.56.tar.gz",
				"zsh-history-substring-search": "https://github.com/zsh-users/zsh-history-substring-search/archive/refs/tags/v1.1.0.tar.gz",
				"powerlevel10k":                "https://github.com/romkatv/powerlevel10k/archive/refs/tags/v1.20.0.tar.gz",
			},
		},
		{
			name: "windows/x86_64",
			os:   "windows", arch: "x86_64",
			want: map[string]string{
				"fd":         "https://github.com/sharkdp/fd/releases/download/v10.5.0/fd-v10.5.0-x86_64-pc-windows-msvc.zip",
				"bat":        "https://github.com/sharkdp/bat/releases/download/v0.26.1/bat-v0.26.1-x86_64-pc-windows-msvc.zip",
				"rg":         "https://github.com/BurntSushi/ripgrep/releases/download/15.2.0/ripgrep-15.2.0-x86_64-pc-windows-msvc.zip",
				"delta":      "https://github.com/dandavison/delta/releases/download/0.19.2/delta-0.19.2-x86_64-pc-windows-msvc.zip",
				"dust":       "https://github.com/bootandy/dust/releases/download/v1.2.6/dust-v1.2.6-x86_64-pc-windows-msvc.zip",
				"eza":        "https://github.com/eza-community/eza/releases/download/v0.23.5/eza.exe_x86_64-pc-windows-gnu.zip",
				"fzf":        "https://github.com/junegunn/fzf/releases/download/v0.74.4/fzf-0.74.4-windows_amd64.zip",
				"lazygit":    "https://github.com/jesseduffield/lazygit/releases/download/v0.65.1/lazygit_0.65.1_Windows_x86_64.zip",
				"age":        "https://github.com/FiloSottile/age/releases/download/v1.3.2/age-v1.3.2-windows-amd64.zip",
				"age-keygen": "https://github.com/FiloSottile/age/releases/download/v1.3.2/age-v1.3.2-windows-amd64.zip",
				"direnv":     "https://github.com/direnv/direnv/releases/download/v2.37.1/direnv.windows-amd64",
				"jq":         "https://github.com/jqlang/jq/releases/download/jq-1.8.2/jq-windows-amd64.exe",
				"nvim":       "https://github.com/neovim/neovim/releases/download/v0.12.5/nvim-win64.zip",
				"go":         "https://go.dev/dl/go1.27.1.windows-amd64.zip",
				"zig":        "https://ziglang.org/download/0.16.0/zig-x86_64-windows-0.16.0.zip",
				"git":        "https://github.com/git-for-windows/git/releases/download/v2.55.0.windows.5/MinGit-2.55.0.5-64-bit.zip",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustURLs(t, tt.os, tt.arch, vers, tt.skip)
			eqURLs(t, got, tt.want)
		})
	}
}

func TestURLsSkipNvim(t *testing.T) {
	got := mustURLs(t, "linux", "x86_64", testVers(), map[string]bool{"nvim": true})
	if _, ok := got["nvim"]; ok {
		t.Fatal("skip nvim still listed nvim")
	}
	if got["fd"] == "" {
		t.Fatal("skip nvim dropped fd")
	}
}

func TestURLsAgeKeysShareURL(t *testing.T) {
	got := mustURLs(t, "linux", "x86_64", testVers(), nil)
	if got["age"] == "" {
		t.Fatal("missing age")
	}
	if got["age"] != got["age-keygen"] {
		t.Fatalf("age=%q age-keygen=%q", got["age"], got["age-keygen"])
	}
}

func TestURLsGitWindowsVersion(t *testing.T) {
	tests := []struct {
		name, arch, ver, want string
	}{
		{
			name: "2.55.0.5 x86_64",
			arch: "x86_64", ver: "2.55.0.5",
			want: "https://github.com/git-for-windows/git/releases/download/v2.55.0.windows.5/MinGit-2.55.0.5-64-bit.zip",
		},
		{
			name: "2.53.0.2 x86_64",
			arch: "x86_64", ver: "2.53.0.2",
			want: "https://github.com/git-for-windows/git/releases/download/v2.53.0.windows.2/MinGit-2.53.0.2-64-bit.zip",
		},
		{
			name: "2.55.0.5 arm64",
			arch: "arm64", ver: "2.55.0.5",
			want: "https://github.com/git-for-windows/git/releases/download/v2.55.0.windows.5/MinGit-2.55.0.5-arm64.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vers := versions.Parse(testEnv + "\nGIT_WINDOWS_VERSION=" + tt.ver)
			got := mustURLs(t, "windows", tt.arch, vers, nil)
			if got["git"] != tt.want {
				t.Errorf("git\n  got  %q\n  want %q", got["git"], tt.want)
			}
		})
	}
}

func TestURLsJqWindowsVsUnix(t *testing.T) {
	unix := mustURLs(t, "linux", "x86_64", testVers(), nil)
	win := mustURLs(t, "windows", "x86_64", testVers(), nil)
	if unix["jq"] != "https://github.com/jqlang/jq/releases/download/jq-1.8.2/jq-linux-amd64" {
		t.Errorf("unix jq = %q", unix["jq"])
	}
	if win["jq"] != "https://github.com/jqlang/jq/releases/download/jq-1.8.2/jq-windows-amd64.exe" {
		t.Errorf("windows jq = %q", win["jq"])
	}
}

var allTools = []string{
	"fd", "bat", "rg", "delta", "dust", "eza",
	"fzf", "lazygit",
	"age", "age-keygen",
	"direnv", "jq",
	"batman", "nvim", "go", "zig", "git", "fzf-shell",
	"zsh-autosuggestions", "zsh-fast-syntax-highlighting",
	"zsh-history-substring-search", "powerlevel10k",
}

func skipAll() map[string]bool {
	m := make(map[string]bool, len(allTools))
	for _, n := range allTools {
		m[n] = true
	}
	return m
}

func TestFetchSkipAll(t *testing.T) {
	p, err := platform.New("linux", "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()

	stdout := captureStdout(t, func() {
		if err := Fetch(out, p, testVers(), skipAll()); err != nil {
			t.Fatal(err)
		}
	})

	binDir := filepath.Join(out, "bin")
	st, err := os.Stat(binDir)
	if err != nil || !st.IsDir() {
		t.Fatalf("bin dir: %v", err)
	}
	entries, err := os.ReadDir(binDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("bin not empty: %v", entries)
	}
	for _, name := range []string{"nvim", "go", "zig", "git", "share"} {
		if _, err := os.Stat(filepath.Join(out, name)); !os.IsNotExist(err) {
			t.Errorf("skip-all created %s", name)
		}
	}
	if !strings.Contains(stdout, "==> Downloading binaries") {
		t.Errorf("missing download banner: %q", stdout)
	}
	if !strings.Contains(stdout, "==> Done:") {
		t.Errorf("missing done line: %q", stdout)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	b, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
