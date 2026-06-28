package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallEzaTheme(t *testing.T) {
	dir := t.TempDir()

	if err := installEzaTheme(dir); err != nil {
		t.Fatalf("installEzaTheme: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "share", "eza", "theme.yml"))
	if err != nil {
		t.Fatalf("read theme: %v", err)
	}

	text := string(content)
	for _, want := range []string{
		`directory: { foreground: "#78a9ff" }`,
		`executable: { foreground: "#25be6a" }`,
		`git_dirty: { foreground: "#be95ff" }`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("theme missing %q\n%s", want, text)
		}
	}
}
