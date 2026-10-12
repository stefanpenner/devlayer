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

func TestPlaceNvimConfigCopiesInit(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "config", "nvim")
	if err := os.MkdirAll(filepath.Join(src, "lua"), 0755); err != nil {
		t.Fatal(err)
	}
	const body = "vim.g.mapleader = \" \"\n"
	if err := os.WriteFile(filepath.Join(src, "init.lua"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	prefix := t.TempDir()
	if err := placeNvimConfig(root, prefix); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(prefix, "share", "devlayer", "config", "nvim", "init.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("init.lua = %q", got)
	}
}

func TestPlaceNvimConfigSkipsAMissingTree(t *testing.T) {
	prefix := t.TempDir()
	if err := placeNvimConfig(t.TempDir(), prefix); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(prefix, "share", "devlayer", "config", "nvim")); !os.IsNotExist(err) {
		t.Fatalf("missing config was copied: %v", err)
	}
}

func TestFindBundleMissing(t *testing.T) {
	_, err := findBundle("no-such-bundle.tar.gz", t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
}
