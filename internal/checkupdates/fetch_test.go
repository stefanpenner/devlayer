package checkupdates

import (
	"fmt"
	"testing"
)

func fixtureGet(url, body string) Get {
	return func(got string) ([]byte, error) {
		if got != url {
			return nil, fmt.Errorf("unexpected url %s", got)
		}
		return []byte(body), nil
	}
}

func TestLatestGitTagsPicksHighestStable(t *testing.T) {
	body := `[
		{"name":"v2.56.0-rc1"},
		{"name":"v2.9.0"},
		{"name":"v2.55.0"},
		{"name":"v2.54.1"},
		{"name":"v2.55.0-rc2"}
	]`
	got := LatestGitTags(fixtureGet("https://api.github.com/repos/git/git/tags?per_page=100", body))
	if got != "2.55.0" {
		t.Errorf("LatestGitTags = %q, want 2.55.0", got)
	}
}

func TestLatestGoJSON(t *testing.T) {
	body := `[
		{"version":"go1.27.1","stable":true},
		{"version":"go1.26.6","stable":true}
	]`
	got := LatestGo(fixtureGet("https://go.dev/dl/?mode=json", body))
	if got != "1.27.1" {
		t.Errorf("LatestGo = %q, want 1.27.1", got)
	}
}

func TestLatestZigIndex(t *testing.T) {
	body := `{
		"master": {},
		"0.15.2": {},
		"0.16.0": {},
		"0.14.1": {}
	}`
	got := LatestZig(fixtureGet("https://ziglang.org/download/index.json", body))
	if got != "0.16.0" {
		t.Errorf("LatestZig = %q, want 0.16.0", got)
	}
}

func TestLatestZshHTML(t *testing.T) {
	body := `<html><body>
<a href="old/">old/</a>
<a href="zsh-5.8.tar.xz">zsh-5.8.tar.xz</a>
<a href="zsh-5.9.tar.xz">zsh-5.9.tar.xz</a>
<a href="zsh-5.9.2.tar.xz">zsh-5.9.2.tar.xz</a>
<a href="zsh-5.9.2-doc.tar.xz">zsh-5.9.2-doc.tar.xz</a>
</body></html>`
	got := LatestZsh(fixtureGet("https://www.zsh.org/pub/", body))
	if got != "5.9.2" {
		t.Errorf("LatestZsh = %q, want 5.9.2", got)
	}
}

func TestLatestGNUHTML(t *testing.T) {
	body := `<a href="make-4.3.tar.gz">make-4.3.tar.gz</a>
<a href="make-4.4.tar.gz">make-4.4.tar.gz</a>
<a href="make-4.4.1.tar.gz">make-4.4.1.tar.gz</a>
<a href="make-4.4.1.tar.gz.sig">make-4.4.1.tar.gz.sig</a>`
	got := LatestGNU(fixtureGet("https://ftp.gnu.org/gnu/make/", body), "make")
	if got != "4.4.1" {
		t.Errorf("LatestGNU = %q, want 4.4.1", got)
	}
}

func TestLatestGitHubUsesTagName(t *testing.T) {
	got := LatestGitHub(
		fixtureGet("https://api.github.com/repos/junegunn/fzf/releases/latest", `{"tag_name":"v0.74.4"}`),
		"junegunn/fzf",
	)
	if got != "0.74.4" {
		t.Errorf("LatestGitHub = %q, want 0.74.4", got)
	}
}

func TestLatestGitWindows(t *testing.T) {
	got := LatestGitWindows(fixtureGet(
		"https://api.github.com/repos/git-for-windows/git/releases/latest",
		`{"tag_name":"v2.55.0.windows.5"}`,
	))
	if got != "2.55.0.5" {
		t.Errorf("LatestGitWindows = %q, want 2.55.0.5", got)
	}
}
