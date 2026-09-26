package stampver_test

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Git describe --dirty refreshes the index. On Windows that call does not
// return, so BazelWorkspaceStatusAction sits at 205/206 after every test
// has passed (run 35561178234, critical path 1004s).

func TestStatusScriptsAvoidIndexRefresh(t *testing.T) {
	root := repoRoot(t)
	want := []string{
		"--no-optional-locks",
		"-c core.fsmonitor=false",
		"describe --tags --always",
	}
	for _, rel := range []string{"tools/workspace_status.sh", "tools/workspace_status.cmd"} {
		text := readRepo(t, root, rel)
		if strings.Contains(codeOnly(text), "--dirty") {
			t.Errorf("%s uses --dirty", rel)
		}
		for _, w := range want {
			if !strings.Contains(text, w) {
				t.Errorf("%s missing %q", rel, w)
			}
		}
	}

	cmd := readRepo(t, root, "tools/workspace_status.cmd")
	if !strings.HasPrefix(cmd, "@echo off") {
		t.Error("workspace_status.cmd must be a cmd script (@echo off)")
	}
	if !strings.Contains(cmd, "<NUL") {
		t.Error("workspace_status.cmd must redirect git stdin from NUL")
	}
	low := strings.ToLower(cmd)
	if strings.Contains(low, "bash") || strings.Contains(cmd, ".sh") {
		t.Error("workspace_status.cmd must not launch bash or a .sh script")
	}
}

func TestWindowsStatusOverridesShell(t *testing.T) {
	text := readRepo(t, repoRoot(t), ".bazelrc")
	shell := strings.Index(text, "build --workspace_status_command=tools/workspace_status.sh")
	enable := strings.Index(text, "build --enable_platform_specific_config")
	win := strings.Index(text, "build:windows --workspace_status_command=tools/workspace_status.cmd")
	if shell < 0 || enable < 0 || win < 0 {
		t.Fatal("bazelrc missing shell status, windows status, or platform config")
	}
	// --config expands where the flag sits. If this is above the shell
	// default, the .sh line wins on Windows and the hang comes back.
	if enable < shell {
		t.Fatal("--enable_platform_specific_config must come after the shell workspace_status_command")
	}
}

func TestPublishNeedsTests(t *testing.T) {
	root := repoRoot(t)
	text := ""
	for _, rel := range []string{".github/workflows/ci.yml", "internal/stampver/ci.yml"} {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err == nil {
			text = string(b)
			break
		}
	}
	if text == "" {
		t.Fatal("ci.yml not found")
	}
	if strings.Contains(text, "Don't let a hung Windows bazel test") {
		t.Fatal("publish still documents skipping the Windows test job")
	}
	if !strings.Contains(text, "needs: [cli, linux, darwin, windows, test]") {
		t.Fatal("publish needs must include test")
	}
}

func TestHostStatusCommandFinishes(t *testing.T) {
	root := repoRoot(t)
	script := filepath.Join(root, "tools/workspace_status.sh")
	if runtime.GOOS == "windows" {
		script = filepath.Join(root, "tools/workspace_status.cmd")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, script)
	cmd.Dir = root
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("workspace status command hung")
	}
	if err != nil {
		t.Fatalf("status command: %v", err)
	}
	line := strings.TrimSpace(string(out))
	ver, ok := strings.CutPrefix(line, "STABLE_VERSION ")
	if !ok || ver == "" || strings.ContainsAny(ver, " \t") {
		t.Fatalf("status line = %q", line)
	}
}

func codeOnly(text string) string {
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if trim == "" || strings.HasPrefix(trim, "#") || strings.HasPrefix(strings.ToLower(trim), "rem ") {
			continue
		}
		b.WriteString(trim)
		b.WriteByte('\n')
	}
	return b.String()
}

func repoRoot(t *testing.T) string {
	t.Helper()
	// Windows Bazel often ships runfiles as a manifest only. Dotfiles are
	// then absent from TEST_SRCDIR, so WalkDir never sees .bazelrc.
	if root, ok := rootFromManifest(); ok {
		return root
	}
	if src := os.Getenv("TEST_SRCDIR"); src != "" {
		var found string
		err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
			if err != nil || found != "" {
				return err
			}
			if !d.IsDir() && d.Name() == ".bazelrc" {
				dir := filepath.Dir(path)
				if _, err := os.Stat(filepath.Join(dir, "tools", "workspace_status.sh")); err != nil {
					return nil
				}
				found = dir
				return fs.SkipAll
			}
			return nil
		})
		if found == "" {
			t.Fatalf(".bazelrc not in TEST_SRCDIR %s (manifest %q): %v", src, os.Getenv("RUNFILES_MANIFEST_FILE"), err)
		}
		return found
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "MODULE.bazel")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("MODULE.bazel not found above %s", file)
		}
		dir = parent
	}
}

func rootFromManifest() (string, bool) {
	manifest := os.Getenv("RUNFILES_MANIFEST_FILE")
	if manifest == "" {
		return "", false
	}
	b, err := os.ReadFile(manifest)
	if err != nil {
		return "", false
	}
	ws := os.Getenv("TEST_WORKSPACE")
	if ws == "" {
		ws = "_main"
	}
	want := ws + "/.bazelrc"
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, "\r")
		rel, abs, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		rel = strings.ReplaceAll(rel, "\\", "/")
		if rel != want {
			continue
		}
		return filepath.Dir(strings.TrimSpace(abs)), true
	}
	return "", false
}

func readRepo(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}
