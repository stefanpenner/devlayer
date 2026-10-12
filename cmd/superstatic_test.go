package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchNvimUnibilium(t *testing.T) {
	in := "" +
		"get_externalproject_options(unibilium ${DEPS_IGNORE_SHA})\n" +
		"ExternalProject_Add(unibilium\n" +
		"  DOWNLOAD_DIR ${DEPS_DOWNLOAD_DIR}/unibilium\n" +
		"  CMAKE_ARGS ${DEPS_CMAKE_ARGS}\n" +
		"  ${EXTERNALPROJECT_OPTIONS})\n"
	got, err := patchNvimUnibilium(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "DEVLAYER_UNIBI_PATCH") || !strings.Contains(got, "-DTERMINFO_DIRS=") {
		t.Fatalf("cmake = %s", got)
	}
	again, err := patchNvimUnibilium(got)
	if err != nil {
		t.Fatal(err)
	}
	if again != got {
		t.Fatal("patch is not idempotent")
	}
}

func TestPatchNvimUnibiliumRejects(t *testing.T) {
	if _, err := patchNvimUnibilium("no CMAKE_ARGS line\n"); err == nil {
		t.Fatal("accepted a cmake file with no unibilium args")
	}
}

func TestWriteEmbeddedTreeModes(t *testing.T) {
	dir := t.TempDir()
	err := WriteEmbeddedTree(dir, map[string]string{
		"third_party/ncurses/configure.sh":  "#!/bin/sh\n",
		"third_party/ncurses/fallbacks.txt": "dumb\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "third_party", "ncurses", "configure.sh")
	info, err := os.Stat(script)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("configure.sh mode = %o", info.Mode().Perm())
	}
	text, err := os.Stat(filepath.Join(dir, "third_party", "ncurses", "fallbacks.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if text.Mode().Perm() != 0o644 {
		t.Fatalf("fallbacks.txt mode = %o", text.Mode().Perm())
	}
}
