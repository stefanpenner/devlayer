// Command packarchive writes a tar.gz.
// file packs one program. tree packs the manifest the Bazel rule already built.
package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var epoch = time.Unix(0, 0).UTC()

var errUsage = errors.New("usage: packarchive file --src SRC --name NAME --out OUT | packarchive tree --manifest MANIFEST --out OUT")

type entry struct {
	kind string
	src  string
	rel  string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, errUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errUsage
	}
	switch args[0] {
	case "file":
		return runFile(args[1:])
	case "tree":
		return runTree(args[1:])
	default:
		return errUsage
	}
}

func runFile(args []string) error {
	fs := flag.NewFlagSet("file", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	src := fs.String("src", "", "file to pack")
	name := fs.String("name", "", "name inside the archive")
	out := fs.String("out", "", "output tar.gz")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	if *src == "" || *name == "" || *out == "" || fs.NArg() != 0 {
		return errUsage
	}
	return writeOut(*out, func(w io.Writer) error {
		return PackFile(*src, *name, w)
	})
}

func runTree(args []string) error {
	fs := flag.NewFlagSet("tree", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	manifest := fs.String("manifest", "", "manifest path")
	out := fs.String("out", "", "output tar.gz")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	if *manifest == "" || *out == "" || fs.NArg() != 0 {
		return errUsage
	}
	f, err := os.Open(*manifest)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeOut(*out, func(w io.Writer) error {
		return PackTree(f, w)
	})
}

func writeOut(path string, fill func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = fill(f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

// PackFile writes src into w as one tar.gz member named name.
// The mode is 0755. Bazel stores action inputs as 0444.
func PackFile(src, name string, w io.Writer) error {
	if err := oneName(name); err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("packarchive: %s is not a file", src)
	}
	return writeGzip(w, func(tw *tar.Writer) error {
		return writeReg(tw, name, f, info.Size(), 0o755)
	})
}

// PackTree reads a manifest of "f|d \t src \t rel" lines and writes a tar.gz.
// Directory lines copy the directory contents. Symlinks are stored as files.
func PackTree(r io.Reader, w io.Writer) error {
	entries, err := parseManifest(r)
	if err != nil {
		return err
	}
	return writeGzip(w, func(tw *tar.Writer) error {
		for _, e := range entries {
			var err error
			if e.kind == "d" {
				err = addDir(tw, e.src, e.rel)
			} else {
				err = addFile(tw, e.src, e.rel)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func parseManifest(r io.Reader) ([]entry, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64), 64*1024)
	var out []entry
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		kind, src, rel, ok := split3(line)
		if !ok {
			return nil, fmt.Errorf("packarchive: bad line")
		}
		if kind != "f" && kind != "d" {
			return nil, fmt.Errorf("packarchive: bad kind %q", kind)
		}
		if src == "" {
			return nil, fmt.Errorf("packarchive: empty src")
		}
		cleaned, err := cleanRel(rel)
		if err != nil {
			return nil, err
		}
		out = append(out, entry{kind: kind, src: src, rel: cleaned})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("packarchive: empty manifest")
	}
	return out, nil
}

func split3(line string) (kind, src, rel string, ok bool) {
	kind, rest, ok := strings.Cut(line, "\t")
	if !ok {
		return "", "", "", false
	}
	src, rel, ok = strings.Cut(rest, "\t")
	if !ok || strings.Contains(rel, "\t") {
		return "", "", "", false
	}
	return kind, src, rel, true
}

func cleanRel(rel string) (string, error) {
	if rel == "" || strings.Contains(rel, "\\") || strings.Contains(rel, "\x00") || strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("packarchive: bad path %q", rel)
	}
	cleaned := path.Clean(rel)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("packarchive: bad path %q", rel)
	}
	return cleaned, nil
}

func oneName(name string) error {
	if name == "" || name == "." || name == ".." || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("packarchive: bad name %q", name)
	}
	return nil
}

func addFile(tw *tar.Writer, src, rel string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("packarchive: %s is not a file", src)
	}
	return writeRegFile(tw, rel, src, int64(info.Mode().Perm()))
}

func addDir(tw *tar.Writer, src, rel string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("packarchive: %s is not a directory", src)
	}
	if err := writeDir(tw, rel); err != nil {
		return err
	}
	return walkFollow(src, func(full, child string, info os.FileInfo) error {
		name := rel + "/" + child
		if info.IsDir() {
			return writeDir(tw, name)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("packarchive: %s is not a file", name)
		}
		return writeRegFile(tw, name, full, int64(info.Mode().Perm()))
	})
}

func walkFollow(root string, fn func(full, rel string, info os.FileInfo) error) error {
	seen := map[string]bool{}
	var rec func(dir, rel string) error
	rec = func(dir, rel string) error {
		real, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return err
		}
		if seen[real] {
			return fmt.Errorf("packarchive: symlink loop %s", dir)
		}
		seen[real] = true
		entries, err := os.ReadDir(real)
		if err != nil {
			return err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, ent := range entries {
			full := filepath.Join(real, ent.Name())
			child := ent.Name()
			if rel != "" {
				child = rel + "/" + ent.Name()
			}
			info, err := os.Stat(full)
			if err != nil {
				return err
			}
			if err := fn(full, child, info); err != nil {
				return err
			}
			if info.IsDir() {
				if err := rec(full, child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return rec(root, "")
}

func writeRegFile(tw *tar.Writer, name, full string, mode int64) error {
	f, err := os.Open(full)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("packarchive: %s is not a file", name)
	}
	return writeReg(tw, name, f, info.Size(), mode)
}

func writeReg(tw *tar.Writer, name string, r io.Reader, size int64, mode int64) error {
	hdr := &tar.Header{
		Name:     name,
		Mode:     mode,
		Size:     size,
		ModTime:  epoch,
		Format:   tar.FormatPAX,
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	n, err := io.Copy(tw, r)
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("packarchive: %s size changed", name)
	}
	return nil
}

func writeDir(tw *tar.Writer, name string) error {
	hdr := &tar.Header{
		Name:     name + "/",
		Mode:     0o755,
		ModTime:  epoch,
		Format:   tar.FormatPAX,
		Typeflag: tar.TypeDir,
	}
	return tw.WriteHeader(hdr)
}

func writeGzip(w io.Writer, fill func(*tar.Writer) error) error {
	gz := gzip.NewWriter(w)
	gz.ModTime = epoch
	gz.OS = 255
	tw := tar.NewWriter(gz)
	err := fill(tw)
	if cerr := tw.Close(); err == nil {
		err = cerr
	}
	if cerr := gz.Close(); err == nil {
		err = cerr
	}
	return err
}
