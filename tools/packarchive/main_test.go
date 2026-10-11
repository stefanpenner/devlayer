package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPackFileForcesExecutable(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in")
	if err := os.WriteFile(src, []byte("make-bytes"), 0o444); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := PackFile(src, "make", &buf); err != nil {
		t.Fatal(err)
	}
	got := readTar(t, buf.Bytes())
	item, ok := got["make"]
	if !ok {
		t.Fatalf("names %v", names(got))
	}
	if item.body != "make-bytes" {
		t.Fatalf("body %q", item.body)
	}
	if item.mode&0o111 != 0o111 {
		t.Fatalf("mode %o", item.mode)
	}
	if item.uid != 0 || !item.mtime.Equal(time.Unix(0, 0).UTC()) {
		t.Fatalf("uid %d mtime %s", item.uid, item.mtime)
	}
}

func TestPackFileFollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.WriteFile(real, []byte("Z"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := PackFile(link, "libz.a", &buf); err != nil {
		t.Fatal(err)
	}
	got := readTar(t, buf.Bytes())
	item := got["libz.a"]
	if item.body != "Z" || item.dir {
		t.Fatalf("body %q dir %v", item.body, item.dir)
	}
}

func TestPackFileStable(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in")
	if err := os.WriteFile(src, []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	var a, b bytes.Buffer
	if err := PackFile(src, "btop", &a); err != nil {
		t.Fatal(err)
	}
	if err := PackFile(src, "btop", &b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("two packs differ")
	}
}

func TestPackTreeLayout(t *testing.T) {
	dir := t.TempDir()
	share := filepath.Join(dir, "share")
	if err := os.MkdirAll(filepath.Join(share, "terminfo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(share, "terminfo", "x"), []byte("term"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("x", filepath.Join(share, "terminfo", "link")); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "git")
	if err := os.WriteFile(bin, []byte("git-bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := strings.Join([]string{
		"f\t" + bin + "\tbin/git",
		"d\t" + share + "\tshare",
		"",
	}, "\n")
	var buf bytes.Buffer
	if err := PackTree(strings.NewReader(manifest), &buf); err != nil {
		t.Fatal(err)
	}
	got := readTar(t, buf.Bytes())
	if got["bin/git"].body != "git-bin" {
		t.Fatalf("git %q", got["bin/git"].body)
	}
	if got["bin/git"].mode&0o111 == 0 {
		t.Fatalf("git mode %o", got["bin/git"].mode)
	}
	if got["share/terminfo/x"].body != "term" {
		t.Fatalf("term %q names %v", got["share/terminfo/x"].body, names(got))
	}
	if got["share/terminfo/link"].body != "term" || got["share/terminfo/link"].dir {
		t.Fatalf("link stored as %+v", got["share/terminfo/link"])
	}
	if _, ok := got["share/share/terminfo/x"]; ok {
		t.Fatal("directory name was packed twice")
	}
	var again bytes.Buffer
	if err := PackTree(strings.NewReader(manifest), &again); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), again.Bytes()) {
		t.Fatal("two tree packs differ")
	}
}

func TestPackTreeKeepsFileMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "note")
	if err := os.WriteFile(src, []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := PackTree(strings.NewReader("f\t"+src+"\tshare/note\n"), &buf); err != nil {
		t.Fatal(err)
	}
	got := readTar(t, buf.Bytes())
	if got["share/note"].mode&0o111 != 0 {
		t.Fatalf("mode %o", got["share/note"].mode)
	}
}

func TestPackTreeSymlinkLoop(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dir, filepath.Join(sub, "loop")); err != nil {
		t.Fatal(err)
	}
	err := PackTree(strings.NewReader("d\t"+dir+"\troot\n"), io.Discard)
	if err == nil {
		t.Fatal("loop returned nil")
	}
}

func TestParseManifestCleansDot(t *testing.T) {
	got, err := parseManifest(strings.NewReader("f\t/src/git\tbin/./git\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].rel != "bin/git" || got[0].src != "/src/git" || got[0].kind != "f" {
		t.Fatalf("%+v", got)
	}
}

func TestParseManifestNeverPanics(t *testing.T) {
	cases := []string{
		"",
		"\n",
		"f",
		"f\t",
		"f\t/src",
		"f\t/src\t",
		"f\t/src\t..\n",
		"f\t/src\t../x\n",
		"f\t/src\t/abs\n",
		"f\t/src\tbin/../../etc/passwd\n",
		"x\t/src\tok\n",
		"f\t\tok\n",
		"f\t/src\tok\textra\n",
		"d\t/src\t.\n",
		"f\t/src\tbin\\git\n",
		string([]byte{0xff, 0xfe}),
		strings.Repeat("A", 70_000),
		"f\t/src\tbin/git\n\n",
	}
	for _, c := range cases {
		func() {
			defer func() {
				if recover() != nil {
					t.Fatalf("panic on %q", c)
				}
			}()
			_, _ = parseManifest(strings.NewReader(c))
		}()
	}
}

func TestRunFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in")
	if err := os.WriteFile(src, []byte("hi"), 0o444); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.tar.gz")
	if err := run([]string{"file", "--src", src, "--name", "make.exe", "--out", out}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := readTar(t, body)
	if got["make.exe"].body != "hi" {
		t.Fatalf("%+v", got["make.exe"])
	}
}

func TestRunUsage(t *testing.T) {
	if err := run(nil); err == nil {
		t.Fatal("missing args")
	}
	if err := run([]string{"nope"}); err == nil {
		t.Fatal("bad command")
	}
	if err := run([]string{"file"}); err == nil {
		t.Fatal("missing flags")
	}
}

func FuzzParseManifest(f *testing.F) {
	f.Add("f\t/src/git\tbin/git\n")
	f.Add("")
	f.Add("d\t/src\t..\n")
	f.Add("f\t/src\tbin/./git\n")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = parseManifest(strings.NewReader(s))
	})
}

type tarItem struct {
	body  string
	mode  int64
	dir   bool
	uid   int
	mtime time.Time
}

func readTar(t *testing.T, gz []byte) map[string]tarItem {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(zr)
	got := map[string]tarItem{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(hdr.Name, "/")
		got[name] = tarItem{
			body:  string(body),
			mode:  hdr.Mode,
			dir:   hdr.Typeflag == tar.TypeDir,
			uid:   hdr.Uid,
			mtime: hdr.ModTime,
		}
	}
	return got
}

func names(got map[string]tarItem) []string {
	out := make([]string, 0, len(got))
	for name := range got {
		out = append(out, name)
	}
	return out
}
