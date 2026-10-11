package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequireBazelCheckout(t *testing.T) {
	dir := t.TempDir()
	err := requireBazelCheckout(dir)
	if err == nil || !strings.Contains(err.Error(), "MODULE.bazel") {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "MODULE.bazel"), []byte("module(name = \"x\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("bazel"); err != nil {
		t.Skip("bazel is not on PATH")
	}
	if err := requireBazelCheckout(dir); err != nil {
		t.Fatal(err)
	}
}
