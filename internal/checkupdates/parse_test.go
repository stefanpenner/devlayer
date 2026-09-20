package checkupdates

import (
	"testing"
)

func TestStripPrefix(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"v1.2.3", "1.2.3"},
		{"jq-1.8.2", "1.8.2"},
		{"go1.27.1", "1.27.1"},
		{"vjq-1.0.0", "1.0.0"},
		{"1.2.3", "1.2.3"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := StripPrefix(tt.in); got != tt.want {
			t.Errorf("StripPrefix(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsRelease(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"1.2.3", true},
		{"2.55.0.5", true},
		{"2024.08.24", true},
		{"foo_bar-1", true},
		{"", false},
		{"1.2.3-rc1", false},
		{"1.2.3-RC.1", false},
		{"1.0.0-alpha", false},
		{"1.0.0-beta", false},
		{"nightly", false},
		{"0.0.0-snapshot", false},
		{"1.0.0+meta", false},
		{"1.0.0/foo", false},
	}
	for _, tt := range tests {
		if got := IsRelease(tt.in); got != tt.want {
			t.Errorf("IsRelease(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestMaxSemver(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"patch", []string{"1.2.3", "1.2.4"}, "1.2.4"},
		{"two-digit minor", []string{"1.10.0", "1.9.9"}, "1.10.0"},
		{"not string sort", []string{"2.9.0", "2.55.0"}, "2.55.0"},
		{"uneven parts", []string{"5.9", "5.9.2", "5.8"}, "5.9.2"},
		{"empty", nil, ""},
		{"zig-like", []string{"0.16.0", "0.15.2"}, "0.16.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaxSemver(tt.in); got != tt.want {
				t.Errorf("MaxSemver(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseGitHubLatestJSON(t *testing.T) {
	got, err := ParseGitHubLatestJSON([]byte(`{"tag_name":"v0.74.4","name":"0.74.4"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "v0.74.4" {
		t.Errorf("tag_name = %q, want v0.74.4", got)
	}

	if _, err := ParseGitHubLatestJSON([]byte(`not json`)); err == nil {
		t.Error("expected error for invalid JSON")
	}

	empty, err := ParseGitHubLatestJSON([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if empty != "" {
		t.Errorf("missing tag_name = %q, want empty", empty)
	}
}

func TestGitWindowsVersion(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"v2.55.0.windows.5", "2.55.0.5"},
		{"2.55.0.windows.5", "2.55.0.5"},
		{"v2.47.1.windows.2", "2.47.1.2"},
	}
	for _, tt := range tests {
		if got := GitWindowsVersion(tt.in); got != tt.want {
			t.Errorf("GitWindowsVersion(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
