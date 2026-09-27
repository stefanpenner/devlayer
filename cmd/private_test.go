package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stefanpenner/devlayer/internal/archive"
	"github.com/stefanpenner/devlayer/internal/config"
)

func TestBuildPrivateMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := t.TempDir()
	if err := buildPrivate(out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, privateTarName)); !os.IsNotExist(err) {
		t.Fatalf("tar should be absent, stat err=%v", err)
	}
}

func TestBuildPrivatePacksCheckout(t *testing.T) {
	origin := filepath.Join(t.TempDir(), "origin.git")
	seed := gitSeed(t, origin)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	dest := filepath.Join(t.TempDir(), "ai-private")
	writePrivateTOML(t, xdg, origin, dest)

	out := t.TempDir()
	if err := buildPrivate(out); err != nil {
		t.Fatal(err)
	}
	assertPacked(t, filepath.Join(out, privateTarName), "marker.txt", "from-seed")

	// A second build fast-forwards the same checkout.
	os.WriteFile(filepath.Join(seed, "marker.txt"), []byte("from-pull\n"), 0644)
	gitCommitAll(t, seed, "update")
	gitRun(t, seed, "push")
	if err := buildPrivate(out); err != nil {
		t.Fatal(err)
	}
	assertPacked(t, filepath.Join(out, privateTarName), "marker.txt", "from-pull")

	home := t.TempDir()
	marker := filepath.Join(home, "ran")
	t.Setenv("MARKER", marker)
	if err := installPrivate(out, home); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ok\n" {
		t.Fatalf("install marker = %q", got)
	}
	cfg, err := config.LoadPrivate()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Path != dest {
		t.Fatalf("path = %q, want %s", cfg.Path, dest)
	}
}

func writePrivateTOML(t *testing.T, xdg, repo, path string) {
	t.Helper()
	dir := filepath.Join(xdg, "devlayer")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	body := "[private]\nrepo = " + quoteTOML(repo) + "\npath = " + quoteTOML(path) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "private.toml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func quoteTOML(s string) string {
	// Single quotes are literal in TOML. A Windows path has \U, which a
	// double-quoted string treats as a Unicode escape.
	return "'" + s + "'"
}

func assertPacked(t *testing.T, tarPath, name, want string) {
	t.Helper()
	dest := t.TempDir()
	if err := archive.ExtractTarGz(tarPath, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, ".ai-private", name))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want+"\n" && string(got) != want {
		t.Fatalf("%s = %q, want %s", name, got, want)
	}
	if _, err := os.Stat(filepath.Join(dest, ".ai-private", ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git should stay out of the tarball, stat err=%v", err)
	}
}

func gitSeed(t *testing.T, bare string) string {
	t.Helper()
	gitRun(t, "", "init", "--bare", "-b", "main", bare)
	seed := t.TempDir()
	gitRun(t, "", "clone", bare, seed)
	script := []byte("#!/bin/sh\necho ok > \"$MARKER\"\n")
	if err := os.WriteFile(filepath.Join(seed, "install.sh"), script, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seed, "marker.txt"), []byte("from-seed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(seed, ".git", "hidden"), 0755); err != nil {
		t.Fatal(err)
	}
	gitCommitAll(t, seed, "seed")
	gitRun(t, seed, "push", "-u", "origin", "main")
	return seed
}

func gitCommitAll(t *testing.T, dir, msg string) {
	t.Helper()
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", msg)
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false"}, args...)...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=devlayer",
		"GIT_AUTHOR_EMAIL=devlayer@example.com",
		"GIT_COMMITTER_NAME=devlayer",
		"GIT_COMMITTER_EMAIL=devlayer@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", args, err, out)
	}
}
