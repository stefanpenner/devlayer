// Package assemble stages a Linux devlayer bundle:
// stage → fetch prebuilts (skip nvim) → extract compiled tars → write wrappers → checksums → pack tar.gz
package assemble

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/stefanpenner/devlayer/internal/archive"
	"github.com/stefanpenner/devlayer/internal/platform"
	"github.com/stefanpenner/devlayer/internal/versions"
)

// Compiled is the set of linuxbuild tar.gz paths extracted into the bundle.
type Compiled struct {
	Git, Zsh, Htop, Btop, Nvim, Make string
}

// Fetch downloads prebuilt tools into out.
type Fetch func(out string, p *platform.Platform, vers *versions.Versions, skip map[string]bool) error

func Run(outputTar, arch string, vers *versions.Versions, compiled Compiled, fetch Fetch) error {
	if fetch == nil {
		return fmt.Errorf("assemble: fetch is required")
	}

	p, err := platform.New("linux", arch)
	if err != nil {
		return err
	}

	staging, err := makeStaging()
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	if err := fetch(staging, p, vers, map[string]bool{"nvim": true}); err != nil {
		return err
	}
	if err := extractCompiled(staging, compiled); err != nil {
		return err
	}
	if err := writeWrappers(filepath.Join(staging, "bin")); err != nil {
		return err
	}
	if err := chmodBin(filepath.Join(staging, "bin")); err != nil {
		return err
	}
	if err := verifyWrappers(filepath.Join(staging, "bin")); err != nil {
		return err
	}
	if err := writeChecksums(staging); err != nil {
		return err
	}
	return archive.CreateTarGz(outputTar, staging)
}

func makeStaging() (string, error) {
	dir, err := os.MkdirTemp("", "devlayer-assemble-*")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0755); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

func extractCompiled(staging string, c Compiled) error {
	bin := filepath.Join(staging, "bin")
	if err := extractTree(c.Git, filepath.Join(staging, "git")); err != nil {
		return fmt.Errorf("git: %w", err)
	}
	if err := extractTree(c.Zsh, filepath.Join(staging, "zsh")); err != nil {
		return fmt.Errorf("zsh: %w", err)
	}
	if err := extractBin(c.Htop, "htop", bin); err != nil {
		return fmt.Errorf("htop: %w", err)
	}
	if err := extractBin(c.Btop, "btop", bin); err != nil {
		return fmt.Errorf("btop: %w", err)
	}
	if err := extractTree(c.Nvim, filepath.Join(staging, "nvim")); err != nil {
		return fmt.Errorf("nvim: %w", err)
	}
	if err := extractBin(c.Make, "make", bin); err != nil {
		return fmt.Errorf("make: %w", err)
	}
	return nil
}

func extractTree(tar, dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	return archive.ExtractTarGz(tar, dest)
}

func extractBin(tar, name, binDir string) error {
	tmp, err := os.MkdirTemp("", "devlayer-assemble-bin-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if err := archive.ExtractTarGz(tar, tmp); err != nil {
		return err
	}
	return copyExecutable(filepath.Join(tmp, name), filepath.Join(binDir, name))
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

func writeWrappers(binDir string) error {
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}
	wrappers := []struct {
		name, body string
	}{
		{"git", gitWrapper()},
		{"zsh", zshWrapper()},
		{"nvim", nvimWrapper()},
		{"go", goWrapper()},
		{"gofmt", gofmtWrapper()},
		{"zig", zigWrapper()},
		{"cc", ccWrapper()},
		{"c++", cxxWrapper()},
	}
	for _, w := range wrappers {
		if err := os.WriteFile(filepath.Join(binDir, w.name), []byte(w.body), 0755); err != nil {
			return fmt.Errorf("wrapper %s: %w", w.name, err)
		}
	}
	return nil
}

func gitWrapper() string {
	return `#!/bin/sh
PREFIX="$(cd "$(dirname "$0")/.." && pwd)"
export GIT_EXEC_PATH="$PREFIX/git/libexec/git-core"
exec "$PREFIX/git/bin/git" "$@"
`
}

func zshWrapper() string {
	return `#!/bin/sh
PREFIX="$(cd "$(dirname "$0")/.." && pwd)"
for d in "$PREFIX"/zsh/share/zsh/*/functions; do [ -d "$d" ] && export FPATH="$d${FPATH:+:$FPATH}" && break; done
exec "$PREFIX/zsh/bin/zsh" "$@"
`
}

func nvimWrapper() string {
	return `#!/bin/sh
PREFIX="$(cd "$(dirname "$0")/.." && pwd)"
export VIMRUNTIME="$PREFIX/nvim/share/nvim/runtime"
exec "$PREFIX/nvim/bin/nvim" "$@"
`
}

func goWrapper() string {
	return `#!/bin/sh
PREFIX="$(cd "$(dirname "$0")/.." && pwd)"
export GOROOT="$PREFIX/go"
exec "$PREFIX/go/bin/go" "$@"
`
}

func gofmtWrapper() string {
	return `#!/bin/sh
PREFIX="$(cd "$(dirname "$0")/.." && pwd)"
export GOROOT="$PREFIX/go"
exec "$PREFIX/go/bin/gofmt" "$@"
`
}

func zigWrapper() string {
	return `#!/bin/sh
PREFIX="$(cd "$(dirname "$0")/.." && pwd)"
exec "$PREFIX/zig/zig" "$@"
`
}

func ccWrapper() string {
	return `#!/bin/sh
PREFIX="$(cd "$(dirname "$0")/.." && pwd)"
exec "$PREFIX/zig/zig" cc "$@"
`
}

func cxxWrapper() string {
	return `#!/bin/sh
PREFIX="$(cd "$(dirname "$0")/.." && pwd)"
exec "$PREFIX/zig/zig" c++ "$@"
`
}

func chmodBin(binDir string) error {
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(binDir, e.Name())
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if err := os.Chmod(path, info.Mode()|0111); err != nil {
			return err
		}
	}
	return nil
}

func verifyWrappers(binDir string) error {
	for _, name := range []string{"git", "zsh", "nvim", "go", "gofmt", "zig", "cc", "c++"} {
		data, err := os.ReadFile(filepath.Join(binDir, name))
		if err != nil {
			return err
		}
		line, _, _ := strings.Cut(string(data), "\n")
		if line != "#!/bin/sh" {
			return fmt.Errorf("FAIL: bin/%s not a wrapper", name)
		}
	}
	return nil
}

func writeChecksums(staging string) error {
	var files []string
	if err := filepath.Walk(staging, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if !checksumFile(info) {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(files)

	var b strings.Builder
	for _, path := range files {
		sum, err := sha256File(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(staging, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "%s  %s\n", sum, rel)
	}
	return os.WriteFile(filepath.Join(staging, "SHA256SUMS"), []byte(b.String()), 0644)
}

func checksumFile(info os.FileInfo) bool {
	if info.Mode()&0111 != 0 {
		return true
	}
	// Windows has no exec bit; hash regular files so tests and stray hosts still get SHA256SUMS.
	return runtime.GOOS == "windows"
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
