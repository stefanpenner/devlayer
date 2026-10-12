package cmd

import (
	"runtime"
	"strings"
	"testing"
)

func TestHtopBuildEnvPrefersSystemPath(t *testing.T) {
	got := htopBuildEnv([]string{"PATH=/Users/stef/.local/bin:/usr/bin"})

	var path string
	for _, entry := range got {
		if strings.HasPrefix(entry, "PATH=") {
			path = strings.TrimPrefix(entry, "PATH=")
			break
		}
	}
	if path == "" {
		t.Fatal("PATH not found")
	}
	if path != "/usr/bin:/bin:/usr/sbin:/sbin" {
		t.Fatalf("unexpected PATH: %s", path)
	}
	if strings.Contains(path, "homebrew") || strings.Contains(path, "/usr/local") {
		t.Fatalf("host package path: %s", path)
	}
	var pkg string
	for _, entry := range got {
		if strings.HasPrefix(entry, "PKG_CONFIG=") {
			pkg = strings.TrimPrefix(entry, "PKG_CONFIG=")
		}
	}
	if pkg != "/usr/bin/false" {
		t.Fatalf("PKG_CONFIG: %q", pkg)
	}
	var flags string
	for _, entry := range got {
		if strings.HasPrefix(entry, "LDFLAGS=") {
			flags = strings.TrimPrefix(entry, "LDFLAGS=")
		}
	}
	if runtime.GOOS == "darwin" && flags != "-Wl,-dead_strip_dylibs" {
		t.Fatalf("LDFLAGS: %q", flags)
	}
	if runtime.GOOS != "darwin" && flags != "" {
		t.Fatalf("LDFLAGS: %q", flags)
	}
}
