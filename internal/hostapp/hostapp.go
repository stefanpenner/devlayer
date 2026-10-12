// Package hostapp names a program that stays on the host.
// The sysroot ships a stub. The stub runs that program after a pin check.
package hostapp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/stefanpenner/devlayer/internal/sums"
)

const (
	// Name is the program the stub runs.
	Name = "terminal-browser"
	// Version is the pinned release.
	Version = "v0.13.4"
	// ExecRel is the program inside the extracted tree.
	ExecRel = "bin/terminal-browser"

	stampName = ".devlayer-sha256"
	baseURL   = "https://terminal-browser.sh/install/dl/stable/" + Version + "/" + Name + "-"
)

// Pin is one archive for one OS and one CPU.
type Pin struct {
	URL string
	Sum string
}

// Decision is the next step for one invocation.
type Decision struct {
	Exec  string
	Fetch bool
	Pin   Pin
	Cache string
}

// Lookup returns the archive for one GOOS and GOARCH.
func Lookup(goos, goarch string) (Pin, error) {
	id, sum, ok := pinFor(goos, goarch)
	if !ok {
		return Pin{}, fmt.Errorf("%s has no pin for %s/%s", Name, goos, goarch)
	}
	if err := sums.Valid(sum); err != nil {
		return Pin{}, err
	}
	return Pin{URL: baseURL + id + ".tar.gz", Sum: sum}, nil
}

func pinFor(goos, goarch string) (id, sum string, ok bool) {
	switch goos + "/" + goarch {
	case "darwin/arm64":
		return "darwin-arm64", "f017230c78c60a07ef4451a1eb0a92727f0b955a8fcd87aec358910c5d0c03c7", true
	case "darwin/amd64":
		return "darwin-x64", "01bc6991bad122f42e4f2a5164a198d8384b944c036de112078fd51f58dc67ed", true
	case "linux/arm64":
		return "linux-arm64", "0cf567d8218995a24fb6ce4b06c07ccf58a8c517906058087355f1e2fad1969f", true
	case "linux/amd64":
		return "linux-x64", "6277daaabab16711ab3f1961cdffad9efac5e70ac55d5076e2c86496d649d3a4", true
	default:
		return "", "", false
	}
}

// Plan chooses a stamped tree, an existing tree, or a download.
func Plan(dataHome, goos, goarch string) (Decision, error) {
	pin, err := Lookup(goos, goarch)
	if err != nil {
		return Decision{}, err
	}
	cache := filepath.Join(dataHome, "devlayer", "host", Name, Version)
	out := Decision{Pin: pin, Cache: cache}
	if stampMatch(cache, pin.Sum) {
		if exec, ok := readyExec(cache, ExecRel); ok {
			out.Exec = exec
			return out, nil
		}
	}
	up := filepath.Join(dataHome, Name, "app")
	if versionMatch(up, Version) {
		if exec, ok := readyExec(up, ExecRel); ok {
			out.Exec = exec
			return out, nil
		}
	}
	out.Fetch = true
	return out, nil
}

// WriteStamp records the pin after the exec file is in the tree.
func WriteStamp(cache, sum string) error {
	if _, ok := readyExec(cache, ExecRel); !ok {
		return fmt.Errorf("%s: %s is missing", Name, ExecRel)
	}
	if err := sums.Valid(sum); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cache, stampName), []byte(sum+"\n"), 0644)
}

// Stub is the sysroot program. It runs the devlayer host command.
func Stub() string {
	return `#!/bin/sh
PREFIX="$(cd "$(dirname "$0")/.." && pwd)"
exec "$PREFIX/bin/devlayer" host terminal-browser "$@"
`
}

func stampMatch(cache, sum string) bool {
	b, ok := readSmall(filepath.Join(cache, stampName), 65)
	if !ok {
		return false
	}
	return strings.TrimRight(string(b), "\n") == sum
}

func versionMatch(dir, want string) bool {
	b, ok := readSmall(filepath.Join(dir, "VERSION"), 64)
	if !ok {
		return false
	}
	got, ok := cleanVersion(b)
	return ok && got == want
}

func cleanVersion(b []byte) (string, bool) {
	if len(b) == 0 || len(b) > 64 {
		return "", false
	}
	s := strings.TrimRight(string(b), "\n")
	if s == "" || strings.TrimSpace(s) != s || strings.ContainsAny(s, "\n\r/\\\x00") {
		return "", false
	}
	return s, true
}

func readSmall(path string, n int) ([]byte, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	b := make([]byte, n+1)
	k, err := f.Read(b)
	if err != nil && err != io.EOF {
		return nil, false
	}
	if k == 0 || k > n {
		return nil, false
	}
	return b[:k], true
}

func readyExec(root, rel string) (string, bool) {
	full, ok := inside(root, rel)
	if !ok {
		return "", false
	}
	info, err := os.Lstat(full)
	if err != nil {
		return "", false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(full)
		if err != nil {
			return "", false
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(full), target)
		}
		if _, ok := inside(root, mustRel(root, target)); !ok {
			return "", false
		}
		full = target
		info, err = os.Stat(full)
		if err != nil {
			return "", false
		}
	}
	if !info.Mode().IsRegular() {
		return "", false
	}
	return full, true
}

func mustRel(root, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return ".."
	}
	return rel
}

func inside(root, rel string) (string, bool) {
	rel = filepath.Clean(rel)
	if rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.Join(root, rel), true
}
