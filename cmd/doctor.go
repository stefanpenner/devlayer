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
	out := []finding{
		{"prefix", dirExists(prefix), prefix},
		{"bin", dirExists(bin), bin},
	}

	pathOK := false
	for _, p := range strings.Split(pathEnv, string(os.PathListSeparator)) {
		if filepath.Clean(p) == filepath.Clean(bin) {
			pathOK = true
			break
		}
	}
	detail := "not on PATH — export PATH=\"" + bin + ":$PATH\""
	if pathOK {
		detail = bin + " is on PATH"
	}
	out = append(out, finding{"PATH", pathOK, detail})

	entries, err := os.ReadDir(bin)
	n := 0
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && !strings.HasPrefix(e.Name(), "._") {
				n++
			}
		}
	}
	out = append(out, finding{"tools", n > 0, fmt.Sprintf("%d files in bin", n)})

	cfg := config.Path()
	out = append(out, finding{"config", fileExists(cfg), cfg})

	nvim := filepath.Join(bin, "nvim")
	if fileExists(nvim) {
		data, _ := os.ReadFile(nvim)
		ok := strings.Contains(string(data), "VIMRUNTIME")
		out = append(out, finding{"nvim", ok, "wrapper sets VIMRUNTIME"})
	}
	goBin := filepath.Join(bin, "go")
	if fileExists(goBin) {
		data, _ := os.ReadFile(goBin)
		ok := strings.Contains(string(data), "GOROOT")
		out = append(out, finding{"go", ok, "wrapper sets GOROOT"})
	}

	return out
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
