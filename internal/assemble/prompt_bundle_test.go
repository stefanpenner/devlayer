package assemble

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	rel = filepath.FromSlash(rel)
	ws := os.Getenv("TEST_WORKSPACE")
	if ws == "" {
		ws = "_main"
	}
	var cands []string
	if src := os.Getenv("TEST_SRCDIR"); src != "" {
		cands = append(cands, filepath.Join(src, ws, rel))
	}
	if wd, err := os.Getwd(); err == nil {
		cands = append(cands, filepath.Join(wd, rel))
	}
	_, here, _, ok := runtime.Caller(0)
	if ok {
		cands = append(cands, filepath.Join(filepath.Dir(here), "..", "..", rel))
	}
	for _, path := range cands {
		b, err := os.ReadFile(path)
		if err == nil {
			return string(b)
		}
	}
	t.Fatalf("could not read %s", rel)
	return ""
}

// Powerlevel10k calls zmodload zsh/files and zsh/regex. A static zsh
// without those modules prints the error and the prompt stays plain.
func TestZshPromptModulesAreStatic(t *testing.T) {
	text := repoFile(t, "tools/extensions.bzl")
	const marker = `n = split("`
	i := strings.Index(text, marker)
	if i < 0 {
		t.Fatal("zsh static module list is missing")
	}
	rest := text[i+len(marker):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatal("zsh static module list is not closed")
	}
	have := map[string]bool{}
	for _, name := range strings.Fields(rest[:end]) {
		have[name] = true
	}
	for _, mod := range []string{"zle", "files", "regex"} {
		if !have[mod] {
			t.Errorf("static zsh module %q is missing; powerlevel10k loads zsh/%s", mod, mod)
		}
	}
}

func TestLinuxBundleShipsNvimConfig(t *testing.T) {
	text := repoFile(t, "linux/defs.bzl")
	const dest = "share/devlayer/config/nvim"
	if !strings.Contains(text, dest) {
		t.Fatalf("linux bundle does not place the nvim config at %s", dest)
	}
}

// gitstatusd must sit next to the plugin. If it is absent, the plugin
// downloads it into the home directory. A noexec home then refuses to run it.
func TestLinuxBundleShipsGitstatusd(t *testing.T) {
	text := repoFile(t, "linux/defs.bzl")
	const dest = "share/powerlevel10k/gitstatus/usrbin/gitstatusd-linux-%s"
	if !strings.Contains(text, dest) {
		t.Fatalf("linux bundle does not place gitstatusd at %s", dest)
	}
}
