package download

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/stefanpenner/devlayer/internal/sums"
)

// File downloads a URL to a local file path after the sha256 pin matches.
func File(url, dest, sum string) error {
	fmt.Printf("  %s\n", filepath.Base(dest))
	tmp, err := verifiedTemp(url, "devlayer-*", sum)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	return copyToFile(tmp, dest)
}

// TarGzBinary downloads a .tar.gz and extracts a single named binary.
func TarGzBinary(url, outputDir, binaryName, sum string) error {
	fmt.Printf("  %s\n", binaryName)
	tmp, err := verifiedTemp(url, "devlayer-*.tar.gz", sum)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	f, err := os.Open(tmp)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip %s: %w", url, err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("%s not found in %s", binaryName, url)
		}
		if err != nil {
			return fmt.Errorf("tar %s: %w", url, err)
		}
		if filepath.Base(hdr.Name) == binaryName && hdr.Typeflag == tar.TypeReg {
			dest := filepath.Join(outputDir, binaryName)
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			return err
		}
	}
}

// TarGzFull downloads a .tar.gz and extracts everything to outputDir.
// If stripComponents > 0, it strips that many leading path components.
func TarGzFull(url, outputDir string, stripComponents int, sum string) error {
	tmp, err := verifiedTemp(url, "devlayer-*.tar.gz", sum)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	return UnpackTarGz(tmp, outputDir, stripComponents)
}

// UnpackTarGz extracts a verified .tar.gz. strip drops that many leading components.
func UnpackTarGz(tarPath, outputDir string, strip int) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip %s: %w", tarPath, err)
	}
	defer gz.Close()
	return unpackTar(gz, outputDir, strip)
}

func unpackTar(r io.Reader, outputDir string, strip int) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		name, ok := stripName(hdr.Name, strip)
		if !ok {
			continue
		}
		if strings.HasPrefix(path.Base(name), "._") {
			continue
		}
		target, err := safeJoin(outputDir, name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)|0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}
}

// ZipBinary downloads a .zip and extracts a single named binary.
func ZipBinary(url, outputDir, binaryName, sum string) error {
	fmt.Printf("  %s\n", binaryName)

	tmp, err := verifiedTemp(url, "devlayer-*.zip", sum)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	info, err := os.Stat(tmp)
	if err != nil {
		return err
	}

	f, err := os.Open(tmp)
	if err != nil {
		return err
	}
	defer f.Close()

	r, err := zip.NewReader(f, info.Size())
	if err != nil {
		return err
	}

	for _, zf := range r.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(zf.Name) == binaryName {
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			dest := filepath.Join(outputDir, binaryName)
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				rc.Close()
				return err
			}
			_, err = io.Copy(out, rc)
			rc.Close()
			out.Close()
			return err
		}
	}
	return fmt.Errorf("%s not found in %s", binaryName, url)
}

// ZipFull downloads a .zip and extracts everything to outputDir.
// If stripComponents > 0, it strips that many leading path components.
func ZipFull(url, outputDir string, stripComponents int, sum string) error {
	tmp, err := verifiedTemp(url, "devlayer-*.zip", sum)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	info, err := os.Stat(tmp)
	if err != nil {
		return err
	}

	f, err := os.Open(tmp)
	if err != nil {
		return err
	}
	defer f.Close()

	r, err := zip.NewReader(f, info.Size())
	if err != nil {
		return err
	}

	for _, zf := range r.File {
		name, ok := stripName(zf.Name, stripComponents)
		if !ok {
			continue
		}
		target, err := safeJoin(outputDir, name)
		if err != nil {
			return err
		}

		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, zf.Mode()|0755)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// ZipFiles downloads a .zip and extracts specified files.
// fileMap maps archive paths (relative) to destination paths (absolute).
func ZipFiles(url string, fileMap map[string]string, sum string) error {
	tmp, err := verifiedTemp(url, "devlayer-*.zip", sum)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	info, err := os.Stat(tmp)
	if err != nil {
		return err
	}

	f, err := os.Open(tmp)
	if err != nil {
		return err
	}
	defer f.Close()

	r, err := zip.NewReader(f, info.Size())
	if err != nil {
		return err
	}

	for _, zf := range r.File {
		dest, ok := fileMap[zf.Name]
		if !ok {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			rc.Close()
			return err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, zf.Mode()|0755)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// TarGzToDir downloads a .tar.gz, extracts it, and renames the top-level
// directory to destDir. Used for plugins where the archive has a single
// top-level directory like "repo-name-version/".
func TarGzToDir(url, destDir, sum string) error {
	tmp, err := os.MkdirTemp("", "devlayer-plugin-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if err := TarGzFull(url, tmp, 0, sum); err != nil {
		return err
	}

	entries, err := os.ReadDir(tmp)
	if err != nil {
		return err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return os.Rename(tmp, destDir)
	}
	return os.Rename(filepath.Join(tmp, entries[0].Name()), destDir)
}

// TarXzFull downloads a .tar.xz and extracts everything to outputDir.
// If stripComponents > 0, it strips that many leading path components.
// Uses the system tar command since Go stdlib doesn't support xz.
func TarXzFull(url, outputDir string, stripComponents int, sum string) error {
	tmp, err := verifiedTemp(url, "devlayer-*.tar.xz", sum)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	args := []string{"xJf", tmp, "-C", outputDir}
	if stripComponents > 0 {
		args = append(args, fmt.Sprintf("--strip-components=%d", stripComponents))
	}

	cmd := exec.Command("tar", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func verifiedTemp(url, pattern, sum string) (string, error) {
	want := strings.ToLower(strings.TrimSpace(sum))
	if err := sums.Valid(want); err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}

	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	fail := func(err error) (string, error) {
		tmp.Close()
		os.Remove(name)
		return "", err
	}

	resp, err := http.Get(url)
	if err != nil {
		return fail(fmt.Errorf("download %s: %w", url, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fail(fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode))
	}

	h := sha256.New()
	if _, err := io.Copy(tmp, io.TeeReader(resp.Body, h)); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		os.Remove(name)
		return "", fmt.Errorf("download %s: sha256 %s, want %s", url, got, want)
	}
	return name, nil
}

func stripName(name string, strip int) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	if strip > 0 {
		parts := strings.SplitN(name, "/", strip+1)
		if len(parts) <= strip {
			return "", false
		}
		name = parts[strip]
	}
	return name, name != ""
}

func safeJoin(dest, name string) (string, error) {
	clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if clean == "." || clean == "" || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	target := filepath.Join(dest, filepath.FromSlash(clean))
	rel, err := filepath.Rel(dest, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return target, nil
}

func copyToFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}
