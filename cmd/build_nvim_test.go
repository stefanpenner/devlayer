package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stefanpenner/devlayer/internal/nvimplugins"
)

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
