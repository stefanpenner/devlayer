package cmd

import (
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
}
