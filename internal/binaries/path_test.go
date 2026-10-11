package binaries

import (
	"strings"
	"testing"
)

// These scripts used to append the host bin directories so configure could
// find ls. The toolbox directory replaces that PATH entry.
func TestActionScriptsDropHostPath(t *testing.T) {
	files := []string{
		"macos/host_make.sh",
		"macos/luajit.sh",
		"macos/archive.sh",
		"third_party/nvim/host_luajit.sh",
		"third_party/nvim/host_nlua0.sh",
	}
	for _, rel := range files {
		text := string(readRepoFile(t, rel))
		if strings.Contains(text, "/bin:/usr/bin") {
			t.Errorf("%s puts a host bin directory on PATH", rel)
		}
	}
}
