package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestBundledBinariesUnix(t *testing.T) {
	got := bundledBinaries("linux")
	for _, want := range []string{"fd", "git", "zsh", "batman", "htop", "ls", "devlayer"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %s", want)
		}
	}
	if slices.Contains(got, "cargo") || slices.Contains(got, "ncurses") {
		t.Errorf("catalog leaked non-bundle name: %v", got)
	}
}

func TestBundledBinariesWindows(t *testing.T) {
	got := bundledBinaries("windows")
	for _, skip := range []string{"zsh", "batman", "htop", "btop", "ls"} {
		if slices.Contains(got, skip) {
			t.Errorf("windows catalog has %s", skip)
		}
	}
	if !slices.Contains(got, "git") || !slices.Contains(got, "fd") {
		t.Errorf("windows missing core tools: %v", got)
	}
}

func TestToolStatusIgnoresStrays(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fd"), []byte("x"), 0755)
	os.WriteFile(filepath.Join(dir, "cargo"), []byte("x"), 0755)
	os.WriteFile(filepath.Join(dir, "rg"), []byte("x"), 0755)

	got := toolStatus(dir, []string{"fd", "rg", "git"})
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	want := map[string]bool{"fd": true, "rg": true, "git": false}
	for _, row := range got {
		if row.Present != want[row.Name] {
			t.Errorf("%s present=%v want %v", row.Name, row.Present, want[row.Name])
		}
	}
}

func TestPluginStatusLockfileOnly(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "oil.nvim"), 0755)
	os.Mkdir(filepath.Join(dir, "random-user-plugin"), 0755)

	got := pluginStatus([]string{"oil.nvim", "mini.nvim"}, dir)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2 (lockfile only)", len(got))
	}
	present := map[string]bool{}
	for _, r := range got {
		present[r.Name] = r.Present
	}
	if !present["oil.nvim"] {
		t.Error("oil.nvim should be present")
	}
	if present["mini.nvim"] {
		t.Error("mini.nvim should be missing")
	}
	if _, ok := present["random-user-plugin"]; ok {
		t.Error("stray plugin leaked into lockfile list")
	}
}
