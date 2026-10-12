package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDarwinWrappersOmitTheHostStub(t *testing.T) {
	dir := t.TempDir()
	if err := writeDarwinWrappers(dir); err != nil {
		t.Fatal(err)
	}
	_, err := os.Stat(filepath.Join(dir, "terminal-browser"))
	if !os.IsNotExist(err) {
		t.Fatal("writeDarwinWrappers wrote terminal-browser")
	}
}

func TestHostAppStaysOutOfTheLinuxBundle(t *testing.T) {
	defs := readHostFile(t, "LINUX_DEFS")
	for _, name := range []string{"terminal-browser", "termium"} {
		if strings.Contains(defs, name) {
			t.Fatalf("linux bundle names %s", name)
		}
	}
}

func TestLsListsTheHostAppOnUnixOnly(t *testing.T) {
	linux := bundledBinaries("linux")
	for _, name := range []string{"terminal-browser", "grok", "agent", "termium"} {
		if !containsName(linux, name) {
			t.Fatalf("linux catalog = %v", linux)
		}
	}
	windows := bundledBinaries("windows")
	for _, name := range []string{"terminal-browser", "grok", "agent", "termium"} {
		if containsName(windows, name) {
			t.Fatalf("windows catalog = %v", windows)
		}
	}
}

func readHostFile(t *testing.T, key string) string {
	t.Helper()
	p := os.Getenv(key)
	if p == "" {
		t.Fatalf("%s is empty", key)
	}
	if _, err := os.Stat(p); err != nil {
		if src := os.Getenv("TEST_SRCDIR"); src != "" {
			ws := os.Getenv("TEST_WORKSPACE")
			if ws == "" {
				ws = "_main"
			}
			alt := src + "/" + ws + "/" + p
			if _, err2 := os.Stat(alt); err2 == nil {
				p = alt
			}
		}
	}
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func containsName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
