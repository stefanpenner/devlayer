package cmd

import (
	"strings"
	"testing"
)

func TestSameVersion(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"v0.3.0", "v0.3.0", true},
		{"v0.3.0", "0.3.0", true},
		{"0.3.0", "v0.3.0", true},
		{"v0.3.0", "v0.4.0", false},
		{"dev", "v0.4.0", false},
		{"", "v0.4.0", false},
	}
	for _, tt := range tests {
		if got := sameVersion(tt.a, tt.b); got != tt.want {
			t.Errorf("sameVersion(%q,%q)=%v want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestFormatNoticeNoBody(t *testing.T) {
	got := formatNotice("v0.3.0", "v0.4.0", "")
	want := "==> New version v0.4.0 (you have v0.3.0)\n    run: devlayer upgrade\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestFormatNoticeWithChangelog(t *testing.T) {
	got := formatNotice("v0.3.0", "v0.4.0", "## What's changed\n\n- hermetic CLIs\n")
	for _, part := range []string{
		"==> New version v0.4.0",
		"run: devlayer upgrade",
		"## What's changed",
		"- hermetic CLIs",
	} {
		if !strings.Contains(got, part) {
			t.Errorf("missing %q in:\n%s", part, got)
		}
	}
}

func TestFormatNoticeSkipsBlankBody(t *testing.T) {
	got := formatNotice("v0.3.0", "v0.4.0", "  \n")
	if strings.Contains(got, "What's changed") || strings.Count(got, "\n") != 2 {
		t.Errorf("blank body should not add notes:\n%q", got)
	}
}
