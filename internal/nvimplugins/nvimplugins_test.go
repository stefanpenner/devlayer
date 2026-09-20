package nvimplugins

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLockfile(t *testing.T) {
	dir := t.TempDir()
	lockfile := filepath.Join(dir, "nvim-pack-lock.json")
	os.WriteFile(lockfile, []byte(`{
  "plugins": {
    "telescope.nvim": { "rev": "abc123", "src": "https://github.com/nvim-telescope/telescope.nvim" },
    "nvim-treesitter": { "rev": "def456", "src": "https://github.com/nvim-treesitter/nvim-treesitter" }
  }
}`), 0644)

	plugins, err := ParseLockfile(lockfile)
	if err != nil {
		t.Fatalf("ParseLockfile: %v", err)
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}

	byName := map[string]Plugin{}
	for _, p := range plugins {
		byName[p.Name] = p
	}

	if p, ok := byName["telescope.nvim"]; !ok {
		t.Error("missing telescope.nvim")
	} else if p.Rev != "abc123" {
		t.Errorf("telescope rev = %s, want abc123", p.Rev)
	}
}

func TestSyncPlugins(t *testing.T) {
	dir := t.TempDir()

	// Create lockfile
	lockfile := filepath.Join(dir, "nvim-pack-lock.json")
	os.WriteFile(lockfile, []byte(`{
  "plugins": {
    "myplugin": { "rev": "aaa", "src": "https://github.com/test/myplugin" },
    "missing": { "rev": "bbb", "src": "https://github.com/test/missing" }
  }
}`), 0644)

	// Create local plugin directory (only myplugin exists)
	localPlugins := filepath.Join(dir, "local-plugins")
	pluginDir := filepath.Join(localPlugins, "myplugin")
	os.MkdirAll(filepath.Join(pluginDir, ".git"), 0755)
	os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte("-- plugin"), 0644)
	os.WriteFile(filepath.Join(pluginDir, ".git", "config"), []byte("gitconfig"), 0644)

	destDir := filepath.Join(dir, "dest")
	err := syncPlugins(lockfile, localPlugins, destDir, func(p Plugin, dest string) error {
		if p.Name == "myplugin" {
			t.Fatalf("fetch should not run for local plugin %s", p.Name)
		}
		return fmt.Errorf("offline")
	})
	if err == nil {
		t.Fatal("expected error when missing plugin cannot be fetched")
	}
}

func TestSyncPluginsCopiesLocal(t *testing.T) {
	dir := t.TempDir()
	lockfile := filepath.Join(dir, "nvim-pack-lock.json")
	os.WriteFile(lockfile, []byte(`{
  "plugins": {
    "myplugin": { "rev": "aaa", "src": "https://github.com/test/myplugin" }
  }
}`), 0644)

	localPlugins := filepath.Join(dir, "local-plugins")
	pluginDir := filepath.Join(localPlugins, "myplugin")
	os.MkdirAll(filepath.Join(pluginDir, ".git"), 0755)
	os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte("-- plugin"), 0644)
	os.WriteFile(filepath.Join(pluginDir, ".git", "config"), []byte("gitconfig"), 0644)

	destDir := filepath.Join(dir, "dest")
	if err := syncPlugins(lockfile, localPlugins, destDir, nil); err != nil {
		t.Fatalf("syncPlugins: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "myplugin", "init.lua")); err != nil {
		t.Error("init.lua not copied")
	}
	if _, err := os.Stat(filepath.Join(destDir, "myplugin", ".git")); !os.IsNotExist(err) {
		t.Error(".git directory should be excluded")
	}
}

func TestSyncPluginsFetchesMissing(t *testing.T) {
	dir := t.TempDir()
	lockfile := filepath.Join(dir, "nvim-pack-lock.json")
	os.WriteFile(lockfile, []byte(`{
  "plugins": {
    "remote": { "rev": "abc", "src": "https://github.com/test/remote" }
  }
}`), 0644)

	destDir := filepath.Join(dir, "dest")
	err := syncPlugins(lockfile, filepath.Join(dir, "empty"), destDir, func(p Plugin, dest string) error {
		if p.Name != "remote" || p.Rev != "abc" {
			t.Fatalf("unexpected plugin %+v", p)
		}
		if err := os.MkdirAll(dest, 0755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dest, "init.lua"), []byte("-- fetched"), 0644)
	})
	if err != nil {
		t.Fatalf("syncPlugins: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "remote", "init.lua")); err != nil {
		t.Error("fetched plugin missing")
	}
}
