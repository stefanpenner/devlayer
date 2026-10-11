package main

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPackStable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.txt", filepath.Join(sub, "link")); err != nil {
		t.Fatal(err)
	}
	long := filepath.Join(dir, strings.Repeat("p", 120))
	if err := os.Mkdir(long, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(long, "f"), []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}

	var first, second bytes.Buffer
	if err := Pack(dir, "MacOSX.sdk", &first); err != nil {
		t.Fatal(err)
	}
	if err := Pack(dir, "MacOSX.sdk", &second); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("two packs of one tree differ")
	}

	tr := tar.NewReader(bytes.NewReader(first.Bytes()))
	saw := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if !hdr.ModTime.Equal(time.Unix(0, 0).UTC()) {
			t.Fatalf("%s mtime %s", hdr.Name, hdr.ModTime)
		}
		if hdr.Uid != 0 || hdr.Gid != 0 {
			t.Fatalf("%s uid %d gid %d", hdr.Name, hdr.Uid, hdr.Gid)
		}
		saw[hdr.Name] = true
	}
	for _, name := range []string{
		"MacOSX.sdk/b.txt",
		"MacOSX.sdk/sub/a.txt",
		"MacOSX.sdk/sub/link",
		"MacOSX.sdk/" + strings.Repeat("p", 120) + "/f",
	} {
		if !saw[name] {
			t.Fatalf("missing %s", name)
		}
	}
}
