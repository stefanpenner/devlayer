package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEzaBuildEnvDarwinUsesCargoAndSystemPath(t *testing.T) {
	got := ezaBuildEnv(nil, "darwin", `/Users/stef/.cargo/bin/cargo`, "/usr/bin/clang", "/usr/bin/clang++", "/sdk")

	want := map[string]bool{
		"CC=/usr/bin/clang":              false,
		"CXX=/usr/bin/clang++":           false,
		"SDKROOT=/sdk":                   false,
		"LDFLAGS=-Wl,-dead_strip_dylibs": false,
		"RUSTFLAGS=-C link-arg=-dead_strip_dylibs -C linker=/usr/bin/clang": false,
		"LIBZ_SYS_STATIC=1":         false,
		"PKG_CONFIG=/usr/bin/false": false,
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

	if path != "/Users/stef/.cargo/bin:/usr/bin:/bin:/usr/sbin:/sbin" {
		t.Fatalf("unexpected PATH: %s", path)
	}
	if strings.Contains(path, "homebrew") || strings.Contains(path, "/usr/local") {
		t.Fatalf("host package path: %s", path)
	}
}

func TestEzaBuildEnvDropsVendoredOverride(t *testing.T) {
	base := []string{"LIBGIT2_NO_VENDOR=1"}
	got := ezaBuildEnv(base, "darwin", `/Users/stef/.cargo/bin/cargo`, "/usr/bin/clang", "/usr/bin/clang++", "/sdk")
	for _, entry := range got {
		if strings.HasPrefix(entry, "LIBGIT2_NO_VENDOR=") {
			t.Fatalf("kept host libgit2 override: %s", entry)
		}
	}
}

func TestEzaCargoArgsVendorsLibgit2(t *testing.T) {
	got := strings.Join(ezaCargoArgs(), " ")
	if got != "build --release --features vendored-libgit2" {
		t.Fatalf("%s", got)
	}
}

func TestEzaCargoEnvOverridesUserTargetDir(t *testing.T) {
	src := filepath.Join(t.TempDir(), "eza")
	base := []string{"CARGO_TARGET_DIR=" + filepath.Join(t.TempDir(), "shared")}
	got := ezaCargoEnv(base, src, "darwin", "/Users/stef/.cargo/bin/cargo", "/usr/bin/clang", "/usr/bin/clang++", "/sdk")

	var dir string
	for _, entry := range got {
		if strings.HasPrefix(entry, "CARGO_TARGET_DIR=") {
			dir = strings.TrimPrefix(entry, "CARGO_TARGET_DIR=")
		}
	}
	if dir != filepath.Join(src, "target") {
		t.Fatalf("target dir = %q", dir)
	}
}

func TestEzaBuildEnvNonDarwinUnchanged(t *testing.T) {
	base := []string{"A=B"}
	got := ezaBuildEnv(base, "linux", "/Users/stef/.cargo/bin/cargo", "/usr/bin/clang", "/usr/bin/clang++", "/sdk")
	if len(got) != 1 || got[0] != "A=B" {
		t.Fatalf("unexpected env: %v", got)
	}
}
