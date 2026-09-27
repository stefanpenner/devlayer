package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stefanpenner/devlayer/internal/config"
)

type finding struct {
	name   string
	ok     bool
	detail string
}

// Doctor checks the local install.
func Doctor() error {
	prefix := defaultPrefix()
	findings := inspectPrefix(prefix, os.Getenv("PATH"))

	var failed int
	for _, f := range findings {
		mark := "ok"
		if !f.ok {
			mark = "FAIL"
			failed++
		}
		fmt.Printf("  %-8s %-8s %s\n", mark, f.name, f.detail)
	}
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	return nil
}

func inspectPrefix(prefix, pathEnv string) []finding {
	bin := filepath.Join(prefix, "bin")
	onPath := binOnPath(bin, pathEnv)
	n := toolCount(bin)
	cfg := config.Path()
	out := []finding{
		{"prefix", dirExists(prefix), prefix},
		{"bin", dirExists(bin), bin},
		{"PATH", onPath, pathDetail(bin, onPath)},
		{"tools", n > 0, fmt.Sprintf("%d files in bin", n)},
		{"config", fileExists(cfg), cfg},
	}

	if nvim := filepath.Join(bin, "nvim"); fileExists(nvim) {
		out = append(out, finding{"nvim", fileContains(nvim, "VIMRUNTIME"), "wrapper sets VIMRUNTIME"})
	}
	if goBin := filepath.Join(bin, "go"); fileExists(goBin) {
		out = append(out, finding{"go", fileContains(goBin, "GOROOT"), "wrapper sets GOROOT"})
	}

	return out
}

func binOnPath(bin, pathEnv string) bool {
	for _, p := range strings.Split(pathEnv, string(os.PathListSeparator)) {
		if filepath.Clean(p) == filepath.Clean(bin) {
			return true
		}
	}
	return false
}

func pathDetail(bin string, onPath bool) string {
	if onPath {
		return bin + " is on PATH"
	}
	return "not on PATH — export PATH=\"" + bin + ":$PATH\""
}

func toolCount(bin string) int {
	entries, err := os.ReadDir(bin)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), "._") {
			n++
		}
	}
	return n
}

func fileContains(p, needle string) bool {
	data, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), needle)
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
