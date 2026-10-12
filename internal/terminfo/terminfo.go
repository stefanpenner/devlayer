// Package terminfo compiles one name list into ncurses.
// The library does not open a host terminfo directory.
package terminfo

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Names reads one terminal name from each line.
// A line that starts with # is a comment. A blank line is skipped.
func Names(text string) ([]string, error) {
	var out []string
	seen := map[string]struct{}{}
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.ContainsAny(line, " \t") {
			return nil, fmt.Errorf("terminfo line %d has extra fields", i+1)
		}
		if !nameOK(line) {
			return nil, fmt.Errorf("bad terminfo name %q", line)
		}
		if _, ok := seen[line]; ok {
			return nil, fmt.Errorf("duplicate terminfo name %q", line)
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("terminfo list is empty")
	}
	return out, nil
}

func nameOK(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case i > 0 && (c == '.' || c == '+' || c == '-'):
		default:
			return false
		}
	}
	return true
}

// StaticOptions is the configure flag set that compiles names into the library.
// --disable-database stops the library from opening a terminfo directory.
// --without-database does not. Do not pass that flag.
func StaticOptions(names []string, tic, infocmp string) []string {
	return []string{
		"--disable-database",
		"--disable-db-install",
		"--with-fallbacks=" + strings.Join(names, ","),
		"--with-tic-path=" + tic,
		"--with-infocmp-path=" + infocmp,
	}
}

// Load reads third_party/ncurses/fallbacks.txt from a checkout.
func Load(repo string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(repo, "third_party", "ncurses", "fallbacks.txt"))
	if err != nil {
		return nil, err
	}
	return Names(string(b))
}

// Check reads the configured tree. The makefile list must match names.
// NCURSES_USE_DATABASE must be 0.
func Check(dir string, names []string) error {
	mk, err := os.ReadFile(filepath.Join(dir, "ncurses", "Makefile"))
	if err != nil {
		return err
	}
	got := fallbackList(string(mk))
	if len(got) != len(names) {
		return fmt.Errorf("FALLBACK_LIST is %q, want %q", got, names)
	}
	for i := range names {
		if got[i] != names[i] {
			return fmt.Errorf("FALLBACK_LIST is %q, want %q", got, names)
		}
	}
	body, err := databaseDefines(dir)
	if err != nil {
		return err
	}
	if !bytes.Contains(body, []byte("#define NCURSES_USE_DATABASE 0")) {
		return fmt.Errorf("NCURSES_USE_DATABASE is not 0")
	}
	if bytes.Contains(body, []byte("#define NCURSES_USE_DATABASE 1")) {
		return fmt.Errorf("NCURSES_USE_DATABASE is 1")
	}
	if bytes.Contains(body, []byte("@NCURSES_USE_DATABASE@")) {
		return fmt.Errorf("NCURSES_USE_DATABASE was not substituted")
	}
	return nil
}

func fallbackList(mk string) []string {
	for _, line := range strings.Split(mk, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "FALLBACK_LIST") {
			continue
		}
		_, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		return strings.Fields(val)
	}
	return nil
}

func databaseDefines(dir string) ([]byte, error) {
	var b bytes.Buffer
	found := false
	for _, sub := range []string{"include", "ncurses"} {
		root := filepath.Join(dir, sub)
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || info.Size() > 2<<20 {
				return nil
			}
			if strings.HasSuffix(path, ".in") || strings.HasSuffix(path, ".hin") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if bytes.Contains(body, []byte("NCURSES_USE_DATABASE")) {
				found = true
				b.Write(body)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if !found {
		return nil, fmt.Errorf("NCURSES_USE_DATABASE is missing")
	}
	return b.Bytes(), nil
}

// Prepare appends aliases.ti and builds host tic and infocmp from the same source.
// The tic directory is beside src, so the caller can delete the parent temp tree.
func Prepare(src, repo string) (tic, infocmp string, names []string, err error) {
	names, err = Load(repo)
	if err != nil {
		return "", "", nil, err
	}
	script := filepath.Join(repo, "third_party", "ncurses", "configure.sh")
	aliases := filepath.Join(repo, "third_party", "ncurses", "aliases.ti")
	if err := runScript(script, "append", src, aliases); err != nil {
		return "", "", nil, err
	}
	dest := filepath.Join(filepath.Dir(src), "host-tic")
	out, err := runScriptOutput(script, "tic", src, dest)
	if err != nil {
		return "", "", nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || lines[0] == "" || lines[1] == "" {
		return "", "", nil, fmt.Errorf("tic did not report two paths")
	}
	return lines[0], lines[1], names, nil
}

// RepoRoot is the checkout that holds MODULE.bazel.
func RepoRoot() (string, error) {
	if d := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); d != "" {
		return d, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "MODULE.bazel")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("MODULE.bazel not found")
		}
		dir = parent
	}
}

func runScript(script string, args ...string) error {
	_, err := runScriptOutput(script, args...)
	return err
}

func runScriptOutput(script string, args ...string) (string, error) {
	cmd := exec.Command(script, args...)
	cmd.Env = hostEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr.String())
	}
	return string(out), nil
}

func hostEnv() []string {
	drop := map[string]bool{
		"TERMINFO":      true,
		"TERMINFO_DIRS": true,
		"LDFLAGS":       true,
	}
	var out []string
	haveCC := false
	haveCXX := false
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || drop[key] {
			continue
		}
		if key == "DEVLAYER_HOST_CC" {
			haveCC = true
		}
		if key == "DEVLAYER_HOST_CXX" {
			haveCXX = true
		}
		out = append(out, entry)
	}
	if !haveCC {
		out = append(out, "DEVLAYER_HOST_CC=clang")
	}
	if !haveCXX {
		out = append(out, "DEVLAYER_HOST_CXX=clang++")
	}
	return out
}
