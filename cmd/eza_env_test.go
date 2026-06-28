package cmd

import (
	"strings"
	"testing"
)

func TestEzaBuildEnvDarwinUsesCargoAndSystemPath(t *testing.T) {
	got := ezaBuildEnv(nil, "darwin", "/Users/stef/.cargo/bin/cargo", "/usr/bin/clang", "/usr/bin/clang++", "/sdk")

	want := map[string]bool{
		"CC=/usr/bin/clang":    false,
		"CXX=/usr/bin/clang++": false,
		"SDKROOT=/sdk":         false,
	}

	var path string
	for _, entry := range got {
		if _, ok := want[entry]; ok {
			want[entry] = true
		}
		if strings.HasPrefix(entry, "PATH=") {
			path = strings.TrimPrefix(entry, "PATH=")
		}
	}

	for key, seen := range want {
		if !seen {
			t.Fatalf("missing %s in env: %v", key, got)
		}
	}

	if path != "/Users/stef/.cargo/bin:/usr/bin:/bin:/opt/homebrew/bin:/usr/sbin:/sbin" {
		t.Fatalf("unexpected PATH: %s", path)
	}
}

func TestEzaBuildEnvNonDarwinUnchanged(t *testing.T) {
	base := []string{"A=B"}
	got := ezaBuildEnv(base, "linux", "/Users/stef/.cargo/bin/cargo", "/usr/bin/clang", "/usr/bin/clang++", "/sdk")
	if len(got) != 1 || got[0] != "A=B" {
		t.Fatalf("unexpected env: %v", got)
	}
}
