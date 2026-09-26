package nvimplugins

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveURL(t *testing.T) {
	const rev = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	got, err := ArchiveURL(Plugin{Name: "myplugin", Rev: rev, Src: "https://github.com/test/myplugin.git"})
	if err != nil {
		t.Fatal(err)
	}
	want := "https://github.com/test/myplugin/archive/" + rev + ".tar.gz"
	if got != want {
		t.Fatalf("url = %s, want %s", got, want)
	}

	if _, err := ArchiveURL(Plugin{Name: "short", Rev: "abc", Src: "https://github.com/test/myplugin"}); err == nil {
		t.Fatal("short rev should fail")
	}
	if _, err := ArchiveURL(Plugin{Name: "other", Rev: rev, Src: "https://gitlab.com/test/myplugin"}); err == nil {
		t.Fatal("non-github src should fail")
	}
}

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
	err := syncPlugins(lockfile, destDir, func(p Plugin, dest string) error {
		return fmt.Errorf("offline")
	})
	if err == nil {
		t.Fatal("expected error when plugin cannot be fetched")
	}
	if _, err := os.Stat(filepath.Join(destDir, "myplugin", "init.lua")); err == nil {
		t.Fatal("local tree was copied")
	}
}

func TestSyncPluginsIgnoresLocalTree(t *testing.T) {
	dir := t.TempDir()
	lockfile := filepath.Join(dir, "nvim-pack-lock.json")
	os.WriteFile(lockfile, []byte(`{
  "plugins": {
    "myplugin": { "rev": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "src": "https://github.com/test/myplugin" }
  }
}`), 0644)

	localPlugins := filepath.Join(dir, "local-plugins")
	pluginDir := filepath.Join(localPlugins, "myplugin")
	os.MkdirAll(filepath.Join(pluginDir, ".git"), 0755)
	os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte("-- local dirty"), 0644)
	os.WriteFile(filepath.Join(pluginDir, ".git", "config"), []byte("gitconfig"), 0644)

	destDir := filepath.Join(dir, "dest")
	err := syncPlugins(lockfile, destDir, func(p Plugin, dest string) error {
		if p.Rev != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
			t.Fatalf("rev = %s", p.Rev)
		}
		if err := os.MkdirAll(dest, 0755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dest, "init.lua"), []byte("-- pinned"), 0644)
	})
	if err != nil {
		t.Fatalf("syncPlugins: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(destDir, "myplugin", "init.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "-- pinned" {
		t.Fatalf("shipped local tree %q, want lockfile rev", got)
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
	err := syncPlugins(lockfile, destDir, func(p Plugin, dest string) error {
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

func TestFetchArchiveInstallsRev(t *testing.T) {
	const rev = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	body := pluginTar(t, "myplugin-"+rev+"/init.lua", "-- from rev")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer srv.Close()

	var gotURL string
	old := httpGet
	httpGet = func(u string) (*http.Response, error) {
		gotURL = u
		return http.Get(srv.URL)
	}
	t.Cleanup(func() { httpGet = old })

	dest := filepath.Join(t.TempDir(), "myplugin")
	err := fetchArchive(Plugin{
		Name: "myplugin",
		Rev:  rev,
		Src:  "https://github.com/test/myplugin",
	}, dest)
	if err != nil {
		t.Fatal(err)
	}
	wantURL := "https://github.com/test/myplugin/archive/" + rev + ".tar.gz"
	if gotURL != wantURL {
		t.Fatalf("url = %s", gotURL)
	}
	got, err := os.ReadFile(filepath.Join(dest, "init.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "-- from rev" {
		t.Fatalf("got %q", got)
	}
}

func pluginTar(t *testing.T, name, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(body))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
