package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLockRegistryAndGit(t *testing.T) {
	text := `
[[package]]
name = "bininfo"
version = "0.1.0"

[[package]]
name = "bitflags"
version = "2.13.2"
source = "registry+https://github.com/rust-lang/crates.io-index"
checksum = "3ded4057c258ba199e2d26386d3af3780957ecaee6c4ef4041c6b4b8b97c0b06"

[[package]]
name = "other"
version = "1.0.0"
source = "git+https://github.com/example/other"
checksum = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
`
	got, err := ParseLock(text)
	if err == nil || !strings.Contains(err.Error(), "git source") {
		t.Fatalf("git source error = %v", err)
	}
	_ = got

	clean := strings.Replace(text, "git+https://github.com/example/other", "registry+https://github.com/rust-lang/crates.io-index", 1)
	crates, err := ParseLock(clean)
	if err != nil {
		t.Fatal(err)
	}
	if len(crates) != 2 || crates[0].Name != "bitflags" {
		t.Fatalf("crates = %+v", crates)
	}
}

func TestVendorWritesChecksum(t *testing.T) {
	crate := Crate{
		Name:     "tiny",
		Version:  "0.1.0",
		Checksum: "",
	}
	body := crateTar(t, "tiny-0.1.0/src/lib.rs", "pub fn n() {}")
	sum := sha256Hex(body)
	crate.Checksum = sum
	out := t.TempDir()
	err := Vendor(out, []Crate{crate}, func(c Crate) ([]byte, error) {
		if CrateURL(c) != "https://static.crates.io/crates/tiny/tiny-0.1.0.crate" {
			t.Fatalf("url %s", CrateURL(c))
		}
		return body, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "tiny", ".cargo-checksum.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Files   map[string]string `json:"files"`
		Package string            `json:"package"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Package != sum {
		t.Fatalf("package checksum %s", doc.Package)
	}
	if doc.Files["src/lib.rs"] == "" {
		t.Fatalf("files %+v", doc.Files)
	}
	if _, err := os.Stat(filepath.Join(out, "ANCHOR")); err != nil {
		t.Fatal(err)
	}
}

func TestVendorRejectsBadChecksum(t *testing.T) {
	body := crateTar(t, "tiny-0.1.0/src/lib.rs", "x")
	err := Vendor(t.TempDir(), []Crate{{
		Name: "tiny", Version: "0.1.0",
		Checksum: "0000000000000000000000000000000000000000000000000000000000000000",
	}}, func(Crate) ([]byte, error) { return body, nil })
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v", err)
	}
}

func crateTar(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	body := []byte(content)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestHostGoPins(t *testing.T) {
	pins := readRepoFile(t, "crate_pins.sha256")
	for _, url := range []string{
		"https://go.dev/dl/go1.27.1.darwin-arm64.tar.gz",
		"https://go.dev/dl/go1.27.1.linux-amd64.tar.gz",
		"https://go.dev/dl/go1.27.1.linux-arm64.tar.gz",
	} {
		if !strings.Contains(pins, url) {
			t.Errorf("missing pin %s", url)
		}
	}
	bzl := readRepoFile(t, "crates.bzl")
	if strings.Contains(bzl, `go_url = "https://go.dev/dl/go1.27.1.darwin-arm64.tar.gz"`) {
		t.Fatal("cratevendor Go URL is Darwin on every host")
	}
	if !strings.Contains(bzl, "_host_go_url(module_ctx.os.name, module_ctx.os.arch)") {
		t.Fatal("cratevendor Go URL does not follow the host")
	}
}

func readRepoFile(t *testing.T, name string) string {
	t.Helper()
	root := os.Getenv("TEST_SRCDIR")
	if root == "" {
		t.Fatal("TEST_SRCDIR is empty")
	}
	var hit string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || hit != "" {
			return walkErr
		}
		if d.IsDir() || d.Name() != name {
			return nil
		}
		if !strings.Contains(filepath.ToSlash(path), "/tools/"+name) {
			return nil
		}
		hit = path
		return fs.SkipAll
	})
	if hit == "" {
		t.Fatalf("%s is missing under %s: %v", name, root, err)
	}
	body, err := os.ReadFile(hit)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
