package cargo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCargoWorkRootExistsOnLinux(t *testing.T) {
	body := readCargoBzl(t)
	if strings.Contains(body, `work="/private/tmp/`) {
		t.Fatal("cargo work directory is /private/tmp on every host")
	}
	if !strings.Contains(body, "tmp_root=/private/tmp") || !strings.Contains(body, "tmp_root=/tmp") {
		t.Fatalf("cargo work directory has no Linux temp root:\n%s", body)
	}
}

func readCargoBzl(t *testing.T) string {
	t.Helper()
	root := os.Getenv("TEST_SRCDIR")
	if root == "" {
		t.Fatal("TEST_SRCDIR is empty")
	}
	var hit string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || hit != "" {
			return walkErr
		}
		if d.IsDir() || d.Name() != "cargo.bzl" {
			return nil
		}
		if !strings.Contains(filepath.ToSlash(path), "/tools/cargo.bzl") {
			return nil
		}
		hit = path
		return filepath.SkipAll
	})
	if hit == "" {
		t.Fatalf("cargo.bzl is missing under %s: %v", root, err)
	}
	raw, err := os.ReadFile(hit)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
