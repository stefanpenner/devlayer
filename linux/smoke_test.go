package linux_test

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stefanpenner/devlayer/internal/loadcheck"
)

func findRunfile(t *testing.T, name string) string {
	t.Helper()
	root := os.Getenv("TEST_SRCDIR")
	var hit string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() == name {
			hit = p
			return filepath.SkipAll
		}
		return nil
	})
	if hit == "" {
		t.Fatalf("runfile %s is missing under %s", name, root)
	}
	return hit
}

func TestBundleLoad(t *testing.T) {
	bundleName := filepath.Base(os.Getenv("DEVLAYER_BUNDLE"))
	if bundleName == "" || bundleName == "." {
		t.Fatal("DEVLAYER_BUNDLE is empty")
	}
	bundle := findRunfile(t, bundleName)
	if err := loadcheck.Archive(bundle); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "linux" {
		t.Log("loadcheck read the bundle. This host does not run Linux binaries. The worker runs them.")
		return
	}
	root := t.TempDir()
	if err := extractTarGz(bundle, root); err != nil {
		t.Fatal(err)
	}
	arch := "x86_64"
	if strings.Contains(bundleName, "aarch64") {
		arch = "aarch64"
	}
	// These programs are static musl. glibc tools stay out.
	// Alpine has no glibc, so an exec of delta or rg would fail there.
	checks := [][]string{
		{"zsh/bin/zsh", "-fc", "print ok"},
		{"git/bin/git", "--version"},
		{"bin/htop", "--version"},
		{"bin/btop", "--version"},
		{"bin/make", "--version"},
		{"nvim/bin/nvim", "--version"},
		{"share/powerlevel10k/gitstatus/usrbin/gitstatusd-linux-" + arch, "--version"},
	}
	for _, args := range checks {
		bin := filepath.Join(root, filepath.FromSlash(args[0]))
		cmd := exec.Command(bin, args[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", args[0], err, out)
		}
	}
}

func extractTarGz(src, dir string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		rel := path.Clean(hdr.Name)
		if rel == "." {
			continue
		}
		if rel == ".." || strings.HasPrefix(rel, "../") || path.IsAbs(hdr.Name) {
			return fmt.Errorf("bundle path escapes: %s", hdr.Name)
		}
		target := filepath.Join(dir, filepath.FromSlash(rel))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			_, cerr := io.Copy(out, tr)
			closeErr := out.Close()
			if cerr != nil {
				return cerr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if path.IsAbs(hdr.Linkname) {
				return fmt.Errorf("bundle link escapes: %s -> %s", hdr.Name, hdr.Linkname)
			}
			dest := path.Clean(path.Join(path.Dir(rel), hdr.Linkname))
			if dest == ".." || strings.HasPrefix(dest, "../") {
				return fmt.Errorf("bundle link escapes: %s -> %s", hdr.Name, hdr.Linkname)
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}
}
