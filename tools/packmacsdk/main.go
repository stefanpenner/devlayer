// Pack writes a macOS SDK tree to a tar whose bytes depend on the file bytes.
package main

import (
	"archive/tar"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

var epoch = time.Unix(0, 0).UTC()

func main() {
	root := flag.String("root", "", "directory to pack")
	prefix := flag.String("prefix", "MacOSX.sdk", "name of the archive root")
	out := flag.String("out", "", "output tar path")
	flag.Parse()
	if *root == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "need -root and -out")
		os.Exit(2)
	}
	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	err = Pack(*root, *prefix, f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Pack writes root into w. Names are sorted. Owner, group, and mtime are zero.
func Pack(root, prefix string, w io.Writer) error {
	tw := tar.NewWriter(w)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		return writeEntry(tw, root, prefix, path, d)
	})
	if err != nil {
		tw.Close()
		return err
	}
	return tw.Close()
}

func writeEntry(tw *tar.Writer, root, prefix, path string, d fs.DirEntry) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	info, err := d.Info()
	if err != nil {
		return err
	}
	name := prefix + "/" + filepath.ToSlash(rel)
	hdr := &tar.Header{
		Name:     name,
		Mode:     int64(info.Mode().Perm()),
		ModTime:  epoch,
		Format:   tar.FormatPAX,
		Typeflag: tar.TypeReg,
	}
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		hdr.Typeflag = tar.TypeSymlink
		hdr.Linkname = target
		return tw.WriteHeader(hdr)
	}
	if d.IsDir() {
		hdr.Typeflag = tar.TypeDir
		hdr.Name = name + "/"
		return tw.WriteHeader(hdr)
	}
	if !mode.IsRegular() {
		return fmt.Errorf("pack %s: not a regular file", path)
	}
	hdr.Size = info.Size()
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}
