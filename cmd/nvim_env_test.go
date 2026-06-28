package cmd

import "testing"

func TestNvimBuildEnvDarwinUsesSystemClang(t *testing.T) {
	got := nvimBuildEnv(nil, "darwin", "/usr/bin/clang", "/usr/bin/clang++", "/sdk")

	want := map[string]bool{
		"CC=/usr/bin/clang":    false,
		"CXX=/usr/bin/clang++": false,
		"SDKROOT=/sdk":         false,
	}

	for _, entry := range got {
		if _, ok := want[entry]; ok {
			want[entry] = true
		}
	}

	for key, seen := range want {
		if !seen {
			t.Fatalf("missing %s in env: %v", key, got)
		}
	}
}

func TestNvimBuildEnvNonDarwinUnchanged(t *testing.T) {
	base := []string{"A=B"}
	got := nvimBuildEnv(base, "linux", "/usr/bin/clang", "/usr/bin/clang++", "/sdk")
	if len(got) != 1 || got[0] != "A=B" {
		t.Fatalf("unexpected env: %v", got)
	}
}
