package nvim

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxHostCCDropsStatic(t *testing.T) {
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	zig := filepath.Join(dir, "zig")
	writeExe(t, zig, "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$HOST_CC_ARGS\"\n")
	writeExe(t, filepath.Join(dir, "uname"), "#!/bin/sh\ncase \"$1\" in\n-s) echo Linux ;;\n-m) echo x86_64 ;;\n*) exit 1 ;;\nesac\n")

	cmd := exec.Command(hostCC(t), "-o", "make_hash", "make_hash.c", "-static")
	cmd.Env = []string{
		"PATH=" + dir + ":/bin:/usr/bin",
		"NVIM_ZIG=" + zig,
		"ZIG_LOCAL_CACHE_DIR=" + filepath.Join(dir, "cache"),
		"ZIG_GLOBAL_CACHE_DIR=" + filepath.Join(dir, "cache"),
		"HOST_CC_ARGS=" + argsPath,
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("host_cc: %v\n%s", err, out)
	}
	body, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "x86_64-linux-gnu") {
		t.Fatalf("target:\n%s", text)
	}
	if !strings.Contains(text, "make_hash.c") {
		t.Fatalf("args:\n%s", text)
	}
	for _, line := range strings.Split(text, "\n") {
		if line == "-static" {
			t.Fatal("linux host cc still passes -static")
		}
	}
}

func hostCC(t *testing.T) string {
	t.Helper()
	root := os.Getenv("TEST_SRCDIR")
	if root == "" {
		t.Fatal("TEST_SRCDIR is empty")
	}
	var hit string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || hit != "" {
			return walkErr
		}
		if d.IsDir() || d.Name() != "host_cc.sh" {
			return nil
		}
		if strings.Contains(filepath.ToSlash(path), "/third_party/nvim/host_cc.sh") {
			hit = path
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if hit == "" {
		t.Fatal("host_cc.sh is missing")
	}
	return hit
}

func writeExe(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}
