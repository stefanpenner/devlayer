package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInspectPrefixMissingBin(t *testing.T) {
	prefix := t.TempDir()
	findings := inspectPrefix(prefix, "")
	var sawTools bool
	for _, f := range findings {
		if f.name == "tools" && !f.ok {
			sawTools = true
		}
	}
	if !sawTools {
		t.Fatal("expected tools check to fail")
	}
}

func TestInspectPrefixFindsBinary(t *testing.T) {
	prefix := t.TempDir()
	bin := filepath.Join(prefix, "bin")
	os.MkdirAll(bin, 0755)
	name := "gh"
	if runtime.GOOS == "windows" {
		name = "gh.exe"
	}
	os.WriteFile(filepath.Join(bin, name), []byte("x"), 0755)

	findings := inspectPrefix(prefix, bin)
	var pathOK, toolsOK bool
	for _, f := range findings {
		if f.name == "PATH" && f.ok {
			pathOK = true
		}
		if f.name == "tools" && f.ok {
			toolsOK = true
		}
	}
	if !pathOK {
		t.Error("PATH should pass when prefix/bin is on PATH")
	}
	if !toolsOK {
		t.Error("tools should pass when bin exists")
	}
}
