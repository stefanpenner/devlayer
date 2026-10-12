package cmd

import (
	"strings"
	"testing"
)

func TestGlowBuildEnv(t *testing.T) {
	got := glowBuildEnv(nil, "/sdk/bin/go", "linux", "amd64", "off", "off", "-mod=vendor")
	want := map[string]bool{
		"GOROOT=/sdk":         false,
		"GOOS=linux":          false,
		"GOARCH=amd64":        false,
		"CGO_ENABLED=0":       false,
		"GOTOOLCHAIN=local":   false,
		"GO111MODULE=on":      false,
		"GOPROXY=off":         false,
		"GOSUMDB=off":         false,
		"GOFLAGS=-mod=vendor": false,
	}
	for _, entry := range got {
		if _, ok := want[entry]; ok {
			want[entry] = true
		}
	}
	var missing []string
	for key, seen := range want {
		if !seen {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("missing %s in %s", strings.Join(missing, ", "), strings.Join(got, "\n"))
	}
}
