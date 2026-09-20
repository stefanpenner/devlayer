package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindBundlePrefersCwd(t *testing.T) {
	cwd := t.TempDir()
	extra := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "devlayer-darwin-arm64.tar.gz"), []byte("cwd"), 0644)
	os.WriteFile(filepath.Join(extra, "devlayer-darwin-arm64.tar.gz"), []byte("extra"), 0644)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}

	got, err := findBundle("devlayer-darwin-arm64.tar.gz", extra)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(got)
	if string(data) != "cwd" {
		t.Errorf("got %q, want cwd copy", data)
	}
}

func TestFindBundleMissing(t *testing.T) {
	_, err := findBundle("no-such-bundle.tar.gz", t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
}
