package nvimplugins

import (
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

	// Sync
	destDir := filepath.Join(dir, "dest")
	err := SyncPlugins(lockfile, localPlugins, destDir)
	if err != nil {
		t.Fatalf("SyncPlugins: %v", err)
	}

	// Verify plugin was copied
	if _, err := os.Stat(filepath.Join(destDir, "myplugin", "init.lua")); err != nil {
		t.Error("init.lua not copied")
	}

	// Verify .git was excluded
	if _, err := os.Stat(filepath.Join(destDir, "myplugin", ".git")); !os.IsNotExist(err) {
		t.Error(".git directory should be excluded")
	}

	// missing plugin should be skipped (no error)
	if _, err := os.Stat(filepath.Join(destDir, "missing")); !os.IsNotExist(err) {
		t.Error("missing plugin should not exist in dest")
	}
}
