// mkimage stages a tree and writes one reproducible EROFS image.
//
//	mkimage -out root.erofs -mkfs <mkfs.erofs> -loader <ld-musl> -libpath <dirs> \
//	    [-tar bundle.tar.gz] [-file dest=src]... [-link dest=target]... [-dir dest]... \
//	    [-files manifest] [-z lz4hc] [-dedupe]
//
// The tarball is the devlayer sysroot. Files and links add the node pieces.
// -files reads a JSON list of {"dest": ..., "src": ...}, for test runfiles.
// Starlark json.encode writes that file.
// -dedupe hardlinks files with equal bytes and mode. The git build installs
// one binary under many names; EROFS keeps one inode for a hardlink.
// mkfs.erofs gets a fixed time, UUID, and owner, so equal trees give equal bytes.
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type pairs []string

func (p *pairs) String() string     { return strings.Join(*p, ",") }
func (p *pairs) Set(v string) error { *p = append(*p, v); return nil }

func main() {
	var files, links, dirs pairs
	out := flag.String("out", "", "image path")
	mkfs := flag.String("mkfs", "", "mkfs.erofs")
	loader := flag.String("loader", "", "musl loader for mkfs.erofs, empty for a native binary")
	libpath := flag.String("libpath", "", "library path for the loader")
	tarball := flag.String("tar", "", "gzip tarball to extract at /")
	manifest := flag.String("files", "", "JSON list of {dest, src}")
	comp := flag.String("z", "lz4hc", "compressor")
	dedupe := flag.Bool("dedupe", false, "hardlink equal files")
	flag.Var(&files, "file", "dest=src, copied with its mode")
	flag.Var(&links, "link", "dest=target symlink")
	flag.Var(&dirs, "dir", "empty directory")
	flag.Parse()
	if *out == "" || *mkfs == "" {
		fail(fmt.Errorf("need -out and -mkfs"))
	}

	stage, err := os.MkdirTemp(filepath.Dir(*out), "stage")
	check(err)
	defer os.RemoveAll(stage)

	if *tarball != "" {
		check(untar(*tarball, stage))
	}
	if *manifest != "" {
		raw, err := os.ReadFile(*manifest)
		check(err)
		var entries []struct{ Dest, Src string }
		check(json.Unmarshal(raw, &entries))
		for _, e := range entries {
			check(copyFile(e.Src, filepath.Join(stage, e.Dest)))
		}
	}
	for _, f := range files {
		dest, src, ok := strings.Cut(f, "=")
		if !ok {
			fail(fmt.Errorf("-file %q", f))
		}
		check(copyFile(src, filepath.Join(stage, dest)))
	}
	for _, d := range dirs {
		check(os.MkdirAll(filepath.Join(stage, d), 0o755))
	}
	for _, l := range links {
		dest, target, ok := strings.Cut(l, "=")
		if !ok {
			fail(fmt.Errorf("-link %q", l))
		}
		p := filepath.Join(stage, dest)
		check(os.MkdirAll(filepath.Dir(p), 0o755))
		os.Remove(p)
		check(os.Symlink(target, p))
	}
	if *dedupe {
		check(hardlinkDups(stage))
	}

	args := []string{
		"-z" + *comp, "-C131072",
		"-T0", "--all-time", "-U", "00000000-0000-0000-0000-000000000000",
		"--all-root", "--quiet", *out, stage,
	}
	var cmd *exec.Cmd
	if *loader != "" {
		cmd = exec.Command(*loader, append([]string{"--library-path", *libpath, *mkfs}, args...)...)
	} else {
		cmd = exec.Command(*mkfs, args...)
	}
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	check(cmd.Run())
}

func untar(path, dst string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(h.Name)
		if name == "." {
			continue
		}
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("%s: entry outside the tree: %s", path, h.Name)
		}
		p := filepath.Join(dst, name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return err
			}
			w, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(h.Mode)&0o777|0o200)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return err
			}
			if err := w.Close(); err != nil {
				return err
			}
			if err := os.Chmod(p, fs.FileMode(h.Mode)&0o777); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(h.Linkname, p); err != nil {
				return err
			}
		case tar.TypeLink:
			if err := os.Link(filepath.Join(dst, filepath.Clean(h.Linkname)), p); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: unsupported entry %s type %c", path, h.Name, h.Typeflag)
		}
	}
}

func copyFile(src, dst string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("%s is a directory", src)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	mode := fs.FileMode(0o644)
	if st.Mode()&0o111 != 0 {
		mode = 0o755
	}
	os.Remove(dst)
	w, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, in); err != nil {
		w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

func hardlinkDups(root string) error {
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(paths)
	first := map[string]string{}
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			return err
		}
		if st.Size() < 4096 {
			continue
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
		key := hex.EncodeToString(h.Sum(nil)) + st.Mode().String()
		if prev, ok := first[key]; ok {
			if err := os.Remove(p); err != nil {
				return err
			}
			if err := os.Link(prev, p); err != nil {
				return err
			}
			continue
		}
		first[key] = p
	}
	return nil
}

func check(err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mkimage:", err)
	os.Exit(1)
}
