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
	if path != "/usr/bin:/bin:/opt/homebrew/bin:/usr/sbin:/sbin" {
		t.Fatalf("unexpected PATH: %s", path)
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
