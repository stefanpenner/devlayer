// cratevendor copies crates.io packages listed in a Cargo.lock.
// Each crate is checked against the lock checksum.
// The output is a cargo vendor directory.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	lockPath := flag.String("lock", "", "Cargo.lock path")
	out := flag.String("out", "", "vendor directory")
	flag.Parse()
	if *lockPath == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: cratevendor --lock Cargo.lock --out vendor")
		os.Exit(2)
	}
	text, err := os.ReadFile(*lockPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	crates, err := ParseLock(string(text))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := Vendor(*out, crates, download); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Crate is one registry package from a lock file.
type Crate struct {
	Name     string
	Version  string
	Checksum string
}

// ParseLock reads registry packages that have a checksum.
// A git source fails the parse.
func ParseLock(text string) ([]Crate, error) {
	var out []Crate
	for _, block := range strings.Split(text, "[[package]]")[1:] {
		c, ok, err := parsePackage(block)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, c)
		}
	}
	return out, nil
}

func parsePackage(block string) (Crate, bool, error) {
	fields := map[string]string{}
	for _, raw := range strings.Split(block, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "[") || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			fields[key] = val[1 : len(val)-1]
		}
	}
	source := fields["source"]
	if strings.Contains(source, "git+") || strings.HasPrefix(source, "git") {
		return Crate{}, false, fmt.Errorf("cratevendor: git source is not pinned: %s %s", fields["name"], source)
	}
	sum := fields["checksum"]
	if sum == "" {
		return Crate{}, false, nil
	}
	if fields["name"] == "" || fields["version"] == "" {
		return Crate{}, false, fmt.Errorf("cratevendor: package is missing a name or a version")
	}
	if len(sum) != 64 {
		return Crate{}, false, fmt.Errorf("cratevendor: bad checksum for %s", fields["name"])
	}
	return Crate{Name: fields["name"], Version: fields["version"], Checksum: sum}, true, nil
}

// CrateURL is the static.crates.io object for one version.
func CrateURL(c Crate) string {
	return fmt.Sprintf("https://static.crates.io/crates/%s/%s-%s.crate", c.Name, c.Name, c.Version)
}

// Vendor extracts each crate under out.
// Two versions of one name use name-version. One version uses the name.
func Vendor(out string, crates []Crate, fetch func(Crate) ([]byte, error)) error {
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	counts := map[string]int{}
	for _, c := range crates {
		counts[c.Name]++
	}
	for _, c := range crates {
		body, err := fetch(c)
		if err != nil {
			return fmt.Errorf("%s: %w", c.Name, err)
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != c.Checksum {
			return fmt.Errorf("%s: checksum mismatch", c.Name)
		}
		dir := c.Name
		if counts[c.Name] > 1 {
			dir = c.Name + "-" + c.Version
		}
		dest := filepath.Join(out, dir)
		if err := extractCrate(body, dest); err != nil {
			return fmt.Errorf("%s: %w", c.Name, err)
		}
		files, err := fileSums(dest)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(struct {
			Files   map[string]string `json:"files"`
			Package string            `json:"package"`
		}{Files: files, Package: c.Checksum})
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dest, ".cargo-checksum.json"), append(payload, '\n'), 0644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(out, "ANCHOR"), []byte("vendor\n"), 0644)
}

func extractCrate(body []byte, dest string) error {
	zr, err := gzip.NewReader(bytes.NewReader(body))
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
		parts := strings.SplitN(rel, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			continue
		}
		rel = parts[1]
		if rel == ".." || strings.HasPrefix(rel, "../") {
			return fmt.Errorf("crate path escapes: %s", hdr.Name)
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0777)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			cerr := f.Close()
			if err != nil {
				return err
			}
			if cerr != nil {
				return cerr
			}
		}
	}
}

func fileSums(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ".cargo-checksum.json" {
			return nil
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
		files[rel] = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func download(c Crate) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(CrateURL(c))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s: %s", CrateURL(c), resp.Status)
	}
	return io.ReadAll(resp.Body)
}
