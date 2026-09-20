// Package checkupdates loads pins, fetches latest per source, skips prerelease, rewrites versions.env, and reports bumps.
package checkupdates

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/stefanpenner/devlayer/internal/versions"
)

type Get func(url string) ([]byte, error)

type Bump struct {
	Key     string
	Current string
	Latest  string
}

type Report struct {
	Bumps []Bump
}

func (r *Report) Any() bool {
	return len(r.Bumps) > 0
}

func Run(envPath string, get Get, dry bool, getenv func(string) string, stdout io.Writer) (*Report, error) {
	content, err := loadEnv(envPath)
	if err != nil {
		return nil, err
	}

	pins := versions.Parse(content)
	bumps := collectBumps(pins, get)

	if !dry {
		if err := writeEnv(envPath, applyBumps(content, bumps)); err != nil {
			return nil, err
		}
	}

	report := &Report{Bumps: bumps}
	if err := writeUpdatesFile(report, getenv); err != nil {
		return nil, err
	}
	if err := appendGitHubOutput(report, getenv); err != nil {
		return nil, err
	}

	printReport(report, stdout)
	return report, nil
}

func collectBumps(pins *versions.Versions, get Get) []Bump {
	var bumps []Bump
	add := func(key, latest string) {
		current := pins.Get(key + "_VERSION")
		if latest == "" || latest == current {
			return
		}
		bumps = append(bumps, Bump{Key: key, Current: current, Latest: latest})
	}

	add("FZF", LatestGitHub(get, "junegunn/fzf"))
	add("FD", LatestGitHub(get, "sharkdp/fd"))
	add("BAT", LatestGitHub(get, "sharkdp/bat"))
	add("EZA", LatestGitHub(get, "eza-community/eza"))
	add("RG", LatestGitHub(get, "BurntSushi/ripgrep"))
	add("DELTA", LatestGitHub(get, "dandavison/delta"))
	add("LAZYGIT", LatestGitHub(get, "jesseduffield/lazygit"))
	add("BAT_EXTRAS", LatestGitHub(get, "eth-p/bat-extras"))
	add("JQ", LatestGitHub(get, "jqlang/jq"))
	add("DIRENV", LatestGitHub(get, "direnv/direnv"))
	add("NVIM", LatestGitHub(get, "neovim/neovim"))
	add("HTOP", LatestGitHub(get, "htop-dev/htop"))
	add("BTOP", LatestGitHub(get, "aristocratos/btop"))
	add("DUST", LatestGitHub(get, "bootandy/dust"))
	add("AGE", LatestGitHub(get, "FiloSottile/age"))

	add("GIT", LatestGitTags(get))
	add("GIT_WINDOWS", LatestGitWindows(get))
	add("GO", LatestGo(get))
	add("ZIG", LatestZig(get))
	add("ZSH", LatestZsh(get))
	add("MAKE", LatestGNU(get, "make"))
	add("NCURSES", LatestGNU(get, "ncurses"))

	return bumps
}

func applyBumps(content string, bumps []Bump) string {
	for _, b := range bumps {
		content = versions.Rewrite(content, b.Key+"_VERSION", b.Latest)
	}
	return content
}

func loadEnv(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func writeEnv(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func writeUpdatesFile(r *Report, getenv func(string) string) error {
	path := getenv("UPDATES_FILE")
	if path == "" {
		path = "/tmp/devlayer-updates.txt"
	}
	return os.WriteFile(path, []byte(bumpLines(r)), 0o644)
}

func appendGitHubOutput(r *Report, getenv func(string) string) error {
	path := getenv("GITHUB_OUTPUT")
	if path == "" {
		return nil
	}
	line := "has_updates=false\n"
	if r.Any() {
		line = "has_updates=true\n"
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = io.WriteString(f, line)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func bumpLines(r *Report) string {
	var b strings.Builder
	for _, bump := range r.Bumps {
		b.WriteString(bump.Key)
		b.WriteString(": ")
		b.WriteString(bump.Current)
		b.WriteString(" → ")
		b.WriteString(bump.Latest)
		b.WriteByte('\n')
	}
	return b.String()
}

func printReport(r *Report, stdout io.Writer) {
	if !r.Any() {
		fmt.Fprintln(stdout, "==> All tools are up to date")
		return
	}
	fmt.Fprintln(stdout, "==> Updates found:")
	fmt.Fprint(stdout, bumpLines(r))
}
