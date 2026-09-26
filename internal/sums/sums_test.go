package sums

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	a := strings.Repeat("ab", 32)
	b := strings.Repeat("cd", 32)
	text := `
# comment
` + a + `  https://example.com/a.tar.gz

` + b + `  https://example.com/b
`
	pins, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	if pins["https://example.com/a.tar.gz"] == "" || pins["https://example.com/b"] == "" {
		t.Fatalf("pins = %#v", pins)
	}
}

func TestParseRejects(t *testing.T) {
	good := strings.Repeat("ab", 32)
	other := strings.Repeat("cd", 32)
	cases := []string{
		"not-a-pin",
		"abcd  https://example.com/a",
		good + "  https://example.com/a extra",
		good + "  https://example.com/a\n" + other + "  https://example.com/a",
	}
	for _, text := range cases {
		if _, err := Parse(text); err == nil {
			t.Errorf("Parse accepted %q", text)
		}
	}
}

func TestMustMissing(t *testing.T) {
	if _, err := Must("https://example.com/not-pinned"); err == nil {
		t.Fatal("missing pin returned a sum")
	}
}

func TestEmbeddedPinsParse(t *testing.T) {
	if _, err := Pins(); err != nil {
		t.Fatal(err)
	}
}
