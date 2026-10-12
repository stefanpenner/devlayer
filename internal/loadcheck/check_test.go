package loadcheck

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxDevlayerPolicyIsStaticElf(t *testing.T) {
	path, err := policyPath("bininfo/policy/linux/bin/devlayer.toml")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !bytes.Contains(body, []byte("format = \"elf\"")) || !bytes.Contains(body, []byte("DT_NEEDED = []")) {
		t.Fatalf("linux CLI policy is not a static ELF:\n%s", text)
	}
	if bytes.Contains(body, []byte("libc.so")) || bytes.Contains(body, []byte("PT_INTERP = \"/")) {
		t.Fatalf("linux CLI policy names a host library:\n%s", text)
	}
}

func policyPath(rel string) (string, error) {
	if _, err := os.Stat(rel); err == nil {
		return rel, nil
	}
	root := os.Getenv("TEST_SRCDIR")
	if root == "" {
		return "", os.ErrNotExist
	}
	ws := os.Getenv("TEST_WORKSPACE")
	if ws == "" {
		ws = "_main"
	}
	for _, cand := range []string{filepath.Join(root, ws, rel), filepath.Join(root, rel)} {
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}
	return "", os.ErrNotExist
}

func TestDevlayerBinaryMatchesPolicy(t *testing.T) {
	bin, err := resolve("DEVLAYER_BIN")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "bin", "devlayer")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, body, 0o755); err != nil {
		t.Fatal(err)
	}
	if len(body) >= 4 && string(body[:4]) == "\x7fELF" {
		policy, err := policyPath("bininfo/policy/linux")
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("BININFO_POLICY", policy)
	}
	if err := File(dst); err != nil {
		t.Fatal(err)
	}
}

func TestTreeSkipsScripts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cc"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("cc", filepath.Join(dir, "c++")); err != nil {
		t.Fatal(err)
	}
	if err := Tree(dir); err != nil {
		t.Fatal(err)
	}
}

func TestTreeRejectsOutsideSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "host")
	if err := os.WriteFile(outside, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "htop")); err != nil {
		t.Fatal(err)
	}
	err := Tree(dir)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("leaves the tree")) {
		t.Fatalf("got %v", err)
	}
}

func TestArchiveRejectsOutsideSymlink(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	hdr := &tar.Header{Name: "bin/htop", Typeflag: tar.TypeSymlink, Linkname: "/usr/bin/htop", Mode: 0o777}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.tar.gz")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Archive(path)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("bin/htop")) || !bytes.Contains([]byte(err.Error()), []byte("leaves the tree")) {
		t.Fatalf("got %v", err)
	}
}

func TestZipNamesTheMember(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	body := IconvELF()
	hdr := &zip.FileHeader{Name: "bin/delta.exe"}
	hdr.SetMode(0o755)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	err = Archive(path)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("bin/delta.exe")) || !bytes.Contains([]byte(err.Error()), []byte("no policy")) {
		t.Fatalf("got %v", err)
	}
}
