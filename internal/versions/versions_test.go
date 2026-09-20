package versions

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		key   string
		want  string
	}{
		{
			name:  "simple key=value",
			input: "FOO=1.2.3",
			key:   "FOO",
			want:  "1.2.3",
		},
		{
			name:  "multiple keys",
			input: "A=1\nB=2\nC=3",
			key:   "B",
			want:  "2",
		},
		{
			name:  "skips comments",
			input: "# comment\nFOO=bar",
			key:   "FOO",
			want:  "bar",
		},
		{
			name:  "skips blank lines",
			input: "\n\nFOO=bar\n\n",
			key:   "FOO",
			want:  "bar",
		},
		{
			name:  "value with dots",
			input: "VER=10.4.2",
			key:   "VER",
			want:  "10.4.2",
		},
		{
			name:  "value with v prefix",
			input: "TAG=v1.20.0",
			key:   "TAG",
			want:  "v1.20.0",
		},
		{
			name:  "value with equals sign",
			input: "KEY=a=b=c",
			key:   "KEY",
			want:  "a=b=c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Parse(tt.input)
			got := v.Get(tt.key)
			if got != tt.want {
				t.Errorf("Get(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestGetMissingKeyPanics(t *testing.T) {
	v := Parse("FOO=bar")

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing key, got none")
		}
	}()

	v.Get("MISSING")
}

func TestParseRealVersionsEnv(t *testing.T) {
	// Simulates the actual versions.env format
	data := `# Tool versions
FZF_VERSION=0.70.0
FD_VERSION=10.4.2
BAT_VERSION=0.26.1
EZA_VERSION=0.23.4
RG_VERSION=15.1.0
DELTA_VERSION=0.18.2
LAZYGIT_VERSION=0.60.0
JQ_VERSION=1.8.1
DIRENV_VERSION=2.37.1
NVIM_VERSION=0.11.6
GO_VERSION=1.26.1
GIT_VERSION=2.53.0
HTOP_VERSION=3.4.1
BAT_EXTRAS_VERSION=2025.03.10
`

	v := Parse(data)

	keys := []struct{ key, want string }{
		{"FZF_VERSION", "0.70.0"},
		{"FD_VERSION", "10.4.2"},
		{"GO_VERSION", "1.26.1"},
		{"BAT_EXTRAS_VERSION", "2025.03.10"},
	}

	for _, k := range keys {
		got := v.Get(k.key)
		if got != k.want {
			t.Errorf("Get(%q) = %q, want %q", k.key, got, k.want)
		}
	}
}

func TestRewritePreservesCommentsAndOrder(t *testing.T) {
	in := `# header
FOO=1

BAR=2
# trailer
BAZ=3
`
	got := Rewrite(in, "BAR", "9")
	want := `# header
FOO=1

BAR=9
# trailer
BAZ=3
`
	if got != want {
		t.Fatalf("Rewrite preserved layout badly:\n got %q\nwant %q", got, want)
	}
}

func TestRewriteMissingKeyAppends(t *testing.T) {
	got := Rewrite("FOO=1\n", "BAR", "2")
	want := "FOO=1\nBAR=2\n"
	if got != want {
		t.Fatalf("Rewrite append: got %q, want %q", got, want)
	}
}
