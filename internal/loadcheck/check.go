// Package loadcheck checks each shipped binary against its policy file.
// bininfo --policy reads ELF, Mach-O, and PE.
// One TOML file is one binary.
// A binary with no file fails.
package loadcheck

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// Tree checks every file under root.
// A symlink whose target leaves root is a host file.
func Tree(root string) error {
	return run(root)
}

// Archive checks every file in a .tar.gz or .zip bundle.
func Archive(path string) error {
	dir, err := os.MkdirTemp("", "loadcheck-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	switch {
	case strings.HasSuffix(path, ".tar.gz"), strings.HasSuffix(path, ".tgz"):
		err = extractTar(path, dir)
	case strings.HasSuffix(path, ".zip"):
		err = extractZip(path, dir)
	default:
		err = fmt.Errorf("loadcheck: unknown archive %s", path)
	}
	if err != nil {
		return err
	}
	return run(dir)
}

// File checks one path. A script passes.
func File(path string) error {
	return run(path)
}

func run(path string) error {
	bin, err := bininfoBin()
	if err != nil {
		return err
	}
	policy, cleanup, err := policyFile()
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, "--policy", policy, path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("loadcheck: %s: %w", path, err)
		}
		return errors.New(msg)
	}
	return nil
}

func bininfoBin() (string, error) {
	if os.Getenv("BININFO") == "" {
		found, err := exec.LookPath("bininfo")
		if err != nil {
			return "", fmt.Errorf("loadcheck: bininfo is missing")
		}
		return found, nil
	}
	return resolve("BININFO")
}

func policyFile() (string, func(), error) {
	if os.Getenv("BININFO_POLICY") == "" {
		return "", nil, fmt.Errorf("loadcheck: policy is missing")
	}
	base, err := resolve("BININFO_POLICY")
	if err != nil {
		return "", nil, err
	}
	if os.Getenv("BININFO_POLICY_OVERLAY") == "" {
		return base, nil, nil
	}
	over, err := resolve("BININFO_POLICY_OVERLAY")
	if err != nil {
		return "", nil, err
	}
	dir, err := mergePolicy(base, over)
	if err != nil {
		return "", nil, err
	}
	return dir, func() { os.RemoveAll(dir) }, nil
}

// mergePolicy copies base, then overlay. An overlay file replaces one base file.
func mergePolicy(base, overlay string) (string, error) {
	dir, err := os.MkdirTemp("", "loadcheck-policy-")
	if err != nil {
		return "", err
	}
	if err := copyPolicyFiles(base, dir); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	if err := copyPolicyFiles(overlay, dir); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

func copyPolicyFiles(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, body, 0o644)
	})
}

func resolve(key string) (string, error) {
	p := os.Getenv(key)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	var cands []string
	if src := os.Getenv("TEST_SRCDIR"); src != "" {
		ws := os.Getenv("TEST_WORKSPACE")
		if ws == "" {
			ws = "_main"
		}
		cands = append(cands, filepath.Join(src, ws, p), filepath.Join(src, p))
	}
	if wd, err := os.Getwd(); err == nil {
		cands = append(cands, filepath.Join(wd, p))
	}
	for _, cand := range cands {
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}
	return "", fmt.Errorf("loadcheck: %s is missing: %s", key, p)
}

func extractTar(src, dir string) error {
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
		dest, err := memberPath(dir, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, dest); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := writeMember(dest, tr); err != nil {
				return err
			}
		}
	}
}

func extractZip(src, dir string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		dest, err := memberPath(dir, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		if f.Mode()&os.ModeSymlink != 0 {
			in, err := io.ReadAll(io.LimitReader(rc, 4096))
			rc.Close()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(string(in), dest); err != nil {
				return err
			}
			continue
		}
		err = writeMember(dest, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeMember(dest string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, r)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func memberPath(dir, name string) (string, error) {
	clean := path.Clean(strings.TrimPrefix(name, "./"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(name) {
		return "", fmt.Errorf("loadcheck: archive member %s leaves the archive", name)
	}
	return filepath.Join(dir, filepath.FromSlash(clean)), nil
}
