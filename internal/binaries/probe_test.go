package binaries

import (
	"strings"
	"testing"
)

func TestProbeAllOK(t *testing.T) {
	urls := map[string]string{"fd": "https://example/fd", "rg": "https://example/rg"}
	err := Probe(urls, func(string) (int, error) { return 200, nil })
	if err != nil {
		t.Fatal(err)
	}
}

func TestProbeHTTPFail(t *testing.T) {
	urls := map[string]string{"fd": "https://example/fd", "rg": "https://example/rg"}
	err := Probe(urls, func(url string) (int, error) {
		if strings.Contains(url, "rg") {
			return 404, nil
		}
		return 200, nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rg") || !strings.Contains(err.Error(), "404") {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), "fd:") {
		t.Fatalf("fd should not be in fail list: %v", err)
	}
}

type probeErr string

func (e probeErr) Error() string { return string(e) }

func TestProbeStatusErrorMsg(t *testing.T) {
	err := Probe(map[string]string{"jq": "https://example/jq"}, func(string) (int, error) {
		return 0, probeErr("timeout")
	})
	if err == nil || !strings.Contains(err.Error(), "jq") || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("got %v", err)
	}
}

func TestProbeTargetsVisitsDefaultPlatforms(t *testing.T) {
	vers := testVers()
	seen := map[string]bool{}
	status := func(url string) (int, error) {
		seen[url] = true
		return 200, nil
	}
	if err := ProbeTargets(vers, status, DefaultProbeTargets()); err != nil {
		t.Fatal(err)
	}
	if len(seen) < 10 {
		t.Fatalf("too few URLs probed: %d", len(seen))
	}
	var fdLinux, ezaWin bool
	for u := range seen {
		if strings.Contains(u, "fd-v") && strings.Contains(u, "linux-musl") {
			fdLinux = true
		}
		if strings.Contains(u, "eza.exe_") && strings.Contains(u, "windows") {
			ezaWin = true
		}
	}
	if !fdLinux {
		t.Fatal("missing linux musl fd URL")
	}
	if !ezaWin {
		t.Fatal("missing windows eza URL")
	}
}

func TestDefaultProbeTargets(t *testing.T) {
	got := DefaultProbeTargets()
	want := []OSArch{
		{"linux", "x86_64"},
		{"linux", "aarch64"},
		{"darwin", "arm64"},
		{"windows", "x86_64"},
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%v want %v", i, got[i], want[i])
		}
	}
}
