package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTouchTree(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	old := time.Unix(1, 0)
	if err := os.Chtimes(file, old, old); err != nil {
		t.Fatal(err)
	}

	if err := touchTree(dir); err != nil {
		t.Fatalf("touchTree: %v", err)
	}

	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().After(old) {
		t.Fatalf("modtime did not advance: %v", info.ModTime())
	}
}
