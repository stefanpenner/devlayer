package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stefanpenner/devlayer/internal/archive"
	"github.com/stefanpenner/devlayer/internal/binaries"
	"github.com/stefanpenner/devlayer/internal/nvimplugins"
)

func TestRewriteNvimDepURLsUsesThePinnedFile(t *testing.T) {
	remote := binaries.NvimDepURLs()["nvim_treesitter_query"]
	in := "TREESITTER_QUERY_URL " + remote + "\nTREESITTER_QUERY_SHA256 abc\n"
	got, err := rewriteNvimDepURLs(in, map[string]string{remote: "/pin/query.tar.gz"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, remote) || !strings.Contains(got, "file:///pin/query.tar.gz") {
		t.Fatalf("url = %s", got)
	}
	if !strings.Contains(got, "TREESITTER_QUERY_SHA256 abc") {
		t.Fatalf("sha = %s", got)
	}
}

func TestBuildNvimPluginsRequiresLock(t *testing.T) {
	err := buildNvimPlugins(t.TempDir())
	if err == nil {
		t.Fatal("missing lockfile skipped")
	}
}

func TestFindScriptDirWritesLock(t *testing.T) {
	t.Setenv("BUILD_WORKING_DIRECTORY", "")
	dir, tmp, err := FindScriptDir("FOO=1\n", "{\"plugins\":{}}\n")
	if err != nil {
		t.Fatal(err)
	}
	if tmp {
		t.Cleanup(func() { os.RemoveAll(dir) })
	}
	if !tmp {
		t.Fatal("expected a temp context, found a repo checkout")
	}
	lock := nvimplugins.RepoLockfile(dir)
	got, err := os.ReadFile(lock)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{\"plugins\":{}}\n" {
		t.Fatalf("lock = %q", got)
	}
	env, err := os.ReadFile(filepath.Join(dir, "versions.env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(env) != "FOO=1\n" {
		t.Fatalf("versions.env = %q", env)
	}
}

func TestStampNvimSysrootKeepsLuaPackOnly(t *testing.T) {
	script, pluginTar := nvimStampInputs(t)

	t.Run("tar", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "devlayer-linux-aarch64.tar.gz")
		writeTinySysroot(t, path, false)
		if err := stampNvimSysroot(path, script, pluginTar); err != nil {
			t.Fatal(err)
		}
		out := t.TempDir()
		if err := archive.ExtractTarGz(path, out); err != nil {
			t.Fatal(err)
		}
		assertStampedNvim(t, out, "bin/nvim")
	})

	t.Run("zip", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "devlayer-windows-x86_64.zip")
		writeTinySysroot(t, path, true)
		if err := stampNvimSysroot(path, script, pluginTar); err != nil {
			t.Fatal(err)
		}
		out := t.TempDir()
		if err := archive.ExtractZip(path, out); err != nil {
			t.Fatal(err)
		}
		assertStampedNvim(t, out, "bin/nvim.cmd")
	})
}

func TestStampNvimSysrootsSkipsSideArchives(t *testing.T) {
	script, pluginTar := nvimStampInputs(t)
	out := t.TempDir()
	packed := mustRead(t, pluginTar)
	if err := os.WriteFile(filepath.Join(out, "devlayer-nvim-plugins.tar.gz"), packed, 0644); err != nil {
		t.Fatal(err)
	}
	side := []byte("side-archive")
	if err := os.WriteFile(filepath.Join(out, "devlayer-dotfiles.tar.gz"), side, 0644); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(out, "devlayer-darwin-arm64.tar.gz")
	writeTinySysroot(t, bundle, false)

	if err := stampNvimSysroots(script, out); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(out, "devlayer-dotfiles.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(side) {
		t.Fatal("side archive was rewritten")
	}
	root := t.TempDir()
	if err := archive.ExtractTarGz(bundle, root); err != nil {
		t.Fatal(err)
	}
	assertStampedNvim(t, root, "bin/nvim")
}

func TestStampNvimSysrootsRequiresThePack(t *testing.T) {
	out := t.TempDir()
	writeTinySysroot(t, filepath.Join(out, "devlayer-linux-x86_64.tar.gz"), false)
	err := stampNvimSysroots(t.TempDir(), out)
	if err == nil {
		t.Fatal("missing plugin pack was accepted")
	}
}

func nvimStampInputs(t *testing.T) (scriptDir, pluginTar string) {
	t.Helper()
	scriptDir = t.TempDir()
	cfg := filepath.Join(scriptDir, "config", "nvim")
	if err := os.MkdirAll(cfg, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "init.lua"), []byte("vim.g.mapleader = \" \"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pack := t.TempDir()
	snack := filepath.Join(pack, "site", "pack", "core", "opt", "snacks.nvim")
	if err := os.MkdirAll(snack, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snack, "init.lua"), []byte("-- snack\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pack, "mason", "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pack, "mason", "bin", "lua-language-server"), []byte("host"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pack, "site", "parser"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pack, "site", "parser", "lua.so"), []byte("host"), 0644); err != nil {
		t.Fatal(err)
	}
	pluginTar = filepath.Join(t.TempDir(), "plugins.tar.gz")
	if err := archive.CreateTarGz(pluginTar, pack); err != nil {
		t.Fatal(err)
	}
	return scriptDir, pluginTar
}

func writeTinySysroot(t *testing.T, path string, zip bool) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "nvim"), []byte("#!/bin/sh\nold\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("keep\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var err error
	if zip {
		err = archive.CreateZip(path, root)
	} else {
		err = archive.CreateTarGz(path, root)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func assertStampedNvim(t *testing.T, root, wrapper string) {
	t.Helper()
	init, err := os.ReadFile(filepath.Join(root, "share", "devlayer", "config", "nvim", "init.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(init), "mapleader") {
		t.Fatalf("init.lua = %q", init)
	}
	snack := filepath.Join(root, "share", "devlayer", "data", "nvim", "site", "pack", "core", "opt", "snacks.nvim", "init.lua")
	if _, err := os.Stat(snack); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "share", "devlayer", "data", "nvim", "mason")); !os.IsNotExist(err) {
		t.Fatalf("mason leaked into the sysroot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "share", "devlayer", "data", "nvim", "site", "parser", "lua.so")); !os.IsNotExist(err) {
		t.Fatalf("host parser leaked into the sysroot: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, wrapper))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "XDG_CONFIG_HOME") || !strings.Contains(string(body), "snacks.nvim") {
		t.Fatalf("wrapper = %q", body)
	}
	keep, err := os.ReadFile(filepath.Join(root, "keep.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(keep) != "keep\n" {
		t.Fatalf("keep.txt = %q", keep)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
