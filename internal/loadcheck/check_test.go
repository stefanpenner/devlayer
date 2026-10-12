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

func TestTreeSkipsThePinnedGoSDK(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "go", "pkg", "tool", "link.exe")
	if err := os.MkdirAll(filepath.Dir(tool), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tool, IconvELF(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Tree(dir); err != nil {
		t.Fatal(err)
	}
}

func TestMinGitPoliciesRecordTheNamedFiles(t *testing.T) {
	for _, rel := range []string{
		"git/usr/lib/ssh/ssh-pkcs11-helper.exe",
		"git/usr/lib/ssh/ssh-sk-helper.exe",
		"git/usr/lib/ssh/ssh-keysign.exe",
		"git/usr/libexec/getprocaddr64.exe",
		"git/usr/libexec/getprocaddr32.exe",
		"git/usr/bin/ssh-add.exe",
		"git/usr/bin/basename.exe",
		"git/usr/bin/cygwin-console-helper.exe",
		"git/usr/bin/wc.exe",
		"git/usr/bin/cut.exe",
		"git/usr/bin/rebase.exe",
		"git/usr/bin/msys-intl-8.dll",
		"git/usr/bin/tr.exe",
		"git/usr/bin/diff.exe",
		"git/usr/bin/grep.exe",
		"git/usr/bin/msys-ncursesw6.dll",
		"git/usr/bin/msys-com_err-1.dll",
		"git/usr/bin/msys-gmp-10.dll",
		"git/usr/bin/newgrp.exe",
		"git/usr/bin/basenc.exe",
		"git/usr/bin/cp.exe",
		"git/usr/bin/sdiff.exe",
		"git/usr/bin/msys-krb5-26.dll",
		"git/usr/bin/env.exe",
		"git/usr/bin/pwd.exe",
		"git/usr/bin/tail.exe",
		"git/usr/bin/readlink.exe",
		"git/usr/bin/ssh.exe",
		"git/usr/bin/msys-wind-0.dll",
		"git/usr/bin/sh.exe",
		"git/usr/bin/msys-sqlite3-0.dll",
		"git/usr/bin/sort.exe",
	} {
		path, err := policyPath("bininfo/policy/" + rel + ".toml")
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte("format = \"pe\"")) || !bytes.Contains(body, []byte("IMAGE_IMPORT_DESCRIPTOR")) {
			t.Fatalf("%s is not a PE policy:\n%s", rel, body)
		}
		if !bytes.Contains(body, []byte("why = ")) {
			t.Fatalf("%s has no why:\n%s", rel, body)
		}
	}
}

func TestTreeChecksABinaryInsideMinGit(t *testing.T) {
	dir := t.TempDir()
	evil := filepath.Join(dir, "git", "usr", "bin", "not-shipped.exe")
	if err := os.MkdirAll(filepath.Dir(evil), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evil, IconvELF(), 0o755); err != nil {
		t.Fatal(err)
	}
	err := Tree(dir)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("git/usr/bin/not-shipped.exe")) || !bytes.Contains([]byte(err.Error()), []byte("no policy")) {
		t.Fatalf("got %v", err)
	}
}

func TestTreeChecksABinaryNextToTheGoSDK(t *testing.T) {
	dir := t.TempDir()
	evil := filepath.Join(dir, "bin", "evil")
	if err := os.MkdirAll(filepath.Dir(evil), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evil, IconvELF(), 0o755); err != nil {
		t.Fatal(err)
	}
	err := Tree(dir)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("bin/evil")) || !bytes.Contains([]byte(err.Error()), []byte("no policy")) {
		t.Fatalf("got %v", err)
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
