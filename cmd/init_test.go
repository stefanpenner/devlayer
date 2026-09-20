package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stefanpenner/devlayer/internal/config"
)

func TestInitWritesConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))

	if err := Init(false); err != nil {
		t.Fatalf("Init: %v", err)
	}
	path := config.Path()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "[dotfiles]") {
		t.Error("missing [dotfiles]")
	}
	if strings.Contains(string(data), "tmux") {
		t.Error("config still mentions tmux")
	}
}

func TestInitRefusesExisting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	if err := Init(false); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	if err := Init(false); err == nil {
		t.Fatal("expected error on existing config")
	}
}

func TestInitForceOverwrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	if err := Init(false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.Path(), []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Init(true); err != nil {
		t.Fatalf("force: %v", err)
	}
	data, _ := os.ReadFile(config.Path())
	if string(data) == "stale" {
		t.Error("did not overwrite")
	}
}
