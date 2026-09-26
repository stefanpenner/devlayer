package download

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileRejectsEmptySum(t *testing.T) {
	err := File("http://127.0.0.1:1/nope", filepath.Join(t.TempDir(), "x"), "")
	if err == nil {
		t.Fatal("empty sum downloaded")
	}
}

func TestFileRejectsMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	err := File(srv.URL, filepath.Join(t.TempDir(), "x"), strings.Repeat("ab", 32))
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("err = %v", err)
	}
}

func TestFileAcceptsMatch(t *testing.T) {
	body := []byte("hello")
	sum := sha256.Sum256(body)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "x")
	if err := File(srv.URL, dest, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("got %q", got)
	}
}

func TestUnpackTarGzStrip(t *testing.T) {
	tarPath := writeTar(t, []tarEntry{{name: "pkg-deadbeef/init.lua", body: "-- hi"}})
	dest := t.TempDir()
	if err := UnpackTarGz(tarPath, dest, 1); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "init.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "-- hi" {
		t.Fatalf("got %q", got)
	}
}

func TestUnpackRejectsDotDot(t *testing.T) {
	tarPath := writeTar(t, []tarEntry{{name: "../evil.txt", body: "x"}})
	err := UnpackTarGz(tarPath, t.TempDir(), 0)
	if err == nil {
		t.Fatal("dotdot path extracted")
	}
}

func TestUnpackSymlink(t *testing.T) {
	tarPath := writeTar(t, []tarEntry{{name: "link", link: "init.lua"}})
	dest := t.TempDir()
	if err := UnpackTarGz(tarPath, dest, 0); err != nil {
		t.Fatal(err)
	}
	got, err := os.Readlink(filepath.Join(dest, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "init.lua" {
		t.Fatalf("link = %s", got)
	}
}

type tarEntry struct {
	name string
	body string
	link string
}

func writeTar(t *testing.T, entries []tarEntry) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: 0644}
		if e.link != "" {
			hdr.Typeflag = tar.TypeSymlink
			hdr.Linkname = e.link
		} else {
			hdr.Typeflag = tar.TypeReg
			hdr.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if e.link == "" {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "a.tar.gz")
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
