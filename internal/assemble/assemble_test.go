package assemble

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stefanpenner/devlayer/internal/archive"
	"github.com/stefanpenner/devlayer/internal/loadcheck"
	"github.com/stefanpenner/devlayer/internal/platform"
	"github.com/stefanpenner/devlayer/internal/versions"
)

func TestWrappers(t *testing.T) {
	prefix := `case "$0" in */*) D="${0%/*}" ;; *) D=. ;; esac; PREFIX="$(cd "$D/.." && pwd)"`
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "git",
			body: gitWrapper(),
			want: []string{
				"#!/bin/sh",
				prefix,
				"GIT_EXEC_PATH=",
				"git/libexec/git-core",
				`exec "$PREFIX/git/bin/git" "$@"`,
			},
		},
		{
			name: "zsh",
			body: zshWrapper(),
			want: []string{
				"#!/bin/sh",
				prefix,
				`for d in "$PREFIX"/zsh/share/zsh/*/functions`,
				`export FPATH="$d${FPATH:+:$FPATH}"`,
				`exec "$PREFIX/zsh/bin/zsh" "$@"`,
			},
		},
		{
			name: "nvim",
			body: NvimWrapper(),
			want: []string{
				"#!/bin/sh",
				prefix,
				"VIMRUNTIME=",
				"nvim/share/nvim/runtime",
				`[ -f "$PREFIX/share/devlayer/config/nvim/init.lua" ]`,
				`[ -d "$PREFIX/share/devlayer/data/nvim/site/pack/core/opt/snacks.nvim" ]`,
				`export XDG_CONFIG_HOME="$PREFIX/share/devlayer/config"`,
				`export XDG_DATA_HOME="$PREFIX/share/devlayer/data"`,
				`exec "$PREFIX/nvim/bin/nvim" "$@"`,
			},
		},
		{
			name: "go",
			body: goWrapper(),
			want: []string{
				"#!/bin/sh",
				prefix,
				"GOROOT=",
				`exec "$PREFIX/go/bin/go" "$@"`,
			},
		},
		{
			name: "gofmt",
			body: gofmtWrapper(),
			want: []string{
				"#!/bin/sh",
				prefix,
				"GOROOT=",
				`exec "$PREFIX/go/bin/gofmt" "$@"`,
			},
		},
		{
			name: "zig",
			body: zigWrapper(),
			want: []string{
				"#!/bin/sh",
				prefix,
				`exec "$PREFIX/zig/zig" "$@"`,
			},
		},
		{
			name: "cc",
			body: ccWrapper(),
			want: []string{
				"#!/bin/sh",
				prefix,
				`exec "$PREFIX/zig/zig" cc "$@"`,
			},
		},
		{
			name: "c++",
			body: cxxWrapper(),
			want: []string{
				"#!/bin/sh",
				prefix,
				`exec "$PREFIX/zig/zig" c++ "$@"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, _, _ := strings.Cut(tt.body, "\n")
			if line != "#!/bin/sh" {
				t.Errorf("first line = %q, want #!/bin/sh", line)
			}
			for _, want := range tt.want {
				if !strings.Contains(tt.body, want) {
					t.Errorf("missing %q\n%s", want, tt.body)
				}
			}
		})
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	compiled := Compiled{
		Git:  oneFileTarGz(t, dir, "git.tar.gz", "bin/git", "git-bin"),
		Zsh:  oneFileTarGz(t, dir, "zsh.tar.gz", "bin/zsh", "zsh-bin"),
		Htop: oneFileTarGz(t, dir, "htop.tar.gz", "htop", "htop-bin"),
		Btop: oneFileTarGz(t, dir, "btop.tar.gz", "btop", "btop-bin"),
		Nvim: oneFileTarGz(t, dir, "nvim.tar.gz", "bin/nvim", "nvim-bin"),
		Make: oneFileTarGz(t, dir, "make.tar.gz", "make", "make-bin"),
	}

	var skip map[string]bool
	fetch := func(out string, p *platform.Platform, vers *versions.Versions, s map[string]bool) error {
		skip = s
		if p == nil || p.OS != "linux" {
			t.Errorf("fetch platform = %+v, want linux", p)
		}
		bin := filepath.Join(out, "bin")
		if err := os.MkdirAll(bin, 0755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(bin, "fd"), []byte("fd"), 0755)
	}

	outTar := filepath.Join(dir, "out.tar.gz")
	if err := Run(outTar, "x86_64", versions.Parse(""), compiled, fetch); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if skip == nil || !skip["nvim"] {
		t.Errorf("skip = %v, want nvim=true", skip)
	}
	if _, err := os.Stat(outTar); err != nil {
		t.Fatalf("output tar: %v", err)
	}

	dst := t.TempDir()
	if err := archive.ExtractTarGz(outTar, dst); err != nil {
		t.Fatalf("extract output: %v", err)
	}

	gitWrap := readFile(t, filepath.Join(dst, "bin", "git"))
	line, _, _ := strings.Cut(gitWrap, "\n")
	if line != "#!/bin/sh" {
		t.Errorf("bin/git first line = %q, want #!/bin/sh", line)
	}
	if got := readFile(t, filepath.Join(dst, "bin", "htop")); got != "htop-bin" {
		t.Errorf("bin/htop = %q, want htop-bin", got)
	}
	if got := readFile(t, filepath.Join(dst, "git", "bin", "git")); got != "git-bin" {
		t.Errorf("git/bin/git = %q, want git-bin", got)
	}

	sums := readFile(t, filepath.Join(dst, "SHA256SUMS"))
	if strings.TrimSpace(sums) == "" {
		t.Error("SHA256SUMS empty")
	}
}

func TestPlaceCopiesPinnedInputs(t *testing.T) {
	dir := t.TempDir()
	binSrc := filepath.Join(dir, "fd")
	if err := os.WriteFile(binSrc, []byte("fd-bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(dir, "sdk")
	if err := os.MkdirAll(filepath.Join(tree, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "BUILD.bazel"), []byte("nope"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "bin", "go"), []byte("go-bytes"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("go", filepath.Join(tree, "bin", "go-link")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "outside-go")
	if err := os.WriteFile(outside, []byte("outside-bytes"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(tree, "bin", "go-abs")); err != nil {
		t.Fatal(err)
	}
	exeSrc := filepath.Join(dir, "gitstatusd")
	if err := os.WriteFile(exeSrc, []byte("daemon"), 0755); err != nil {
		t.Fatal(err)
	}
	fileSrc := filepath.Join(dir, "key-bindings.zsh")
	if err := os.WriteFile(fileSrc, []byte("bind"), 0644); err != nil {
		t.Fatal(err)
	}

	staging := t.TempDir()
	err := Place(staging, Input{
		Bins:  []Bin{{Name: "fd", Path: binSrc}},
		Trees: []Tree{{Dest: "go", Path: tree}},
		Files: []File{
			{Dest: "share/fzf/key-bindings.zsh", Path: fileSrc},
			{Dest: "share/powerlevel10k/gitstatus/usrbin/gitstatusd", Path: exeSrc},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, filepath.Join(staging, "bin", "fd")); got != "fd-bytes" {
		t.Errorf("bin/fd = %q", got)
	}
	info, err := os.Stat(filepath.Join(staging, "bin", "fd"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("bin/fd mode = %v, want executable", info.Mode())
	}
	if _, err := os.Stat(filepath.Join(staging, "go", "BUILD.bazel")); !os.IsNotExist(err) {
		t.Fatalf("BUILD.bazel in tree: %v", err)
	}
	if got := readFile(t, filepath.Join(staging, "go", "bin", "go")); got != "go-bytes" {
		t.Errorf("go/bin/go = %q", got)
	}
	link, err := os.Readlink(filepath.Join(staging, "go", "bin", "go-link"))
	if err != nil {
		t.Fatal(err)
	}
	if link != "go" {
		t.Errorf("symlink = %q, want go", link)
	}
	absInfo, err := os.Lstat(filepath.Join(staging, "go", "bin", "go-abs"))
	if err != nil {
		t.Fatal(err)
	}
	if absInfo.Mode()&os.ModeSymlink != 0 {
		t.Error("external symlink was kept as a link")
	}
	if got := readFile(t, filepath.Join(staging, "go", "bin", "go-abs")); got != "outside-bytes" {
		t.Errorf("external symlink copied as %q", got)
	}
	if got := readFile(t, filepath.Join(staging, "share", "fzf", "key-bindings.zsh")); got != "bind" {
		t.Errorf("fzf binding = %q", got)
	}
	daemon := filepath.Join(staging, "share", "powerlevel10k", "gitstatus", "usrbin", "gitstatusd")
	info, err = os.Stat(daemon)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("gitstatusd mode = %v, want executable", info.Mode())
	}
}

func TestPlaceRejectsEscape(t *testing.T) {
	err := Place(t.TempDir(), Input{Files: []File{{Dest: "../out", Path: "x"}}})
	if err == nil {
		t.Fatal("expected error for dest that leaves the staging dir")
	}
}

func TestParseNamed(t *testing.T) {
	name, path, err := ParseNamed("share/fzf/a.zsh=/pin/a.zsh")
	if err != nil {
		t.Fatal(err)
	}
	if name != "share/fzf/a.zsh" || path != "/pin/a.zsh" {
		t.Fatalf("got %s = %s", name, path)
	}
	if _, _, err := ParseNamed("no-equals"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunPlacedDoesNotFetch(t *testing.T) {
	dir := t.TempDir()
	fd := filepath.Join(dir, "fd")
	if err := os.WriteFile(fd, []byte("fd-bytes"), 0755); err != nil {
		t.Fatal(err)
	}
	compiled := Compiled{
		Git:  oneFileTarGz(t, dir, "git.tar.gz", "bin/git", "git-bin"),
		Zsh:  oneFileTarGz(t, dir, "zsh.tar.gz", "bin/zsh", "zsh-bin"),
		Htop: oneFileTarGz(t, dir, "htop.tar.gz", "htop", "htop-bin"),
		Btop: oneFileTarGz(t, dir, "btop.tar.gz", "btop", "btop-bin"),
		Nvim: oneFileTarGz(t, dir, "nvim.tar.gz", "bin/nvim", "nvim-bin"),
		Make: oneFileTarGz(t, dir, "make.tar.gz", "make", "make-bin"),
	}
	outTar := filepath.Join(dir, "out.tar.gz")
	err := RunPlaced(outTar, "aarch64", compiled, Input{
		Bins: []Bin{{Name: "fd", Path: fd}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := archive.ExtractTarGz(outTar, dst); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dst, "bin", "fd")); got != "fd-bytes" {
		t.Errorf("bin/fd = %q", got)
	}
	line, _, _ := strings.Cut(readFile(t, filepath.Join(dst, "bin", "git")), "\n")
	if line != "#!/bin/sh" {
		t.Errorf("bin/git = %q", line)
	}
}

func TestRunPlacedRejectsIconv(t *testing.T) {
	dir := t.TempDir()
	fd := filepath.Join(dir, "fd")
	if err := os.WriteFile(fd, loadcheck.IconvELF(), 0755); err != nil {
		t.Fatal(err)
	}
	compiled := Compiled{
		Git:  oneFileTarGz(t, dir, "git.tar.gz", "bin/git", "git-bin"),
		Zsh:  oneFileTarGz(t, dir, "zsh.tar.gz", "bin/zsh", "zsh-bin"),
		Htop: oneFileTarGz(t, dir, "htop.tar.gz", "htop", "htop-bin"),
		Btop: oneFileTarGz(t, dir, "btop.tar.gz", "btop", "btop-bin"),
		Nvim: oneFileTarGz(t, dir, "nvim.tar.gz", "bin/nvim", "nvim-bin"),
		Make: oneFileTarGz(t, dir, "make.tar.gz", "make", "make-bin"),
	}
	err := RunPlaced(filepath.Join(dir, "out.tar.gz"), "aarch64", compiled, Input{
		Bins: []Bin{{Name: "fd", Path: fd}},
	})
	if err == nil || !strings.Contains(err.Error(), "bin/fd") || !strings.Contains(err.Error(), "format: found elf") {
		t.Fatalf("got %v", err)
	}
}

func TestMissingFetch(t *testing.T) {
	err := Run(filepath.Join(t.TempDir(), "out.tar.gz"), "x86_64", versions.Parse(""), Compiled{}, nil)
	if err == nil {
		t.Fatal("expected error for nil fetch")
	}
}

func TestBadArch(t *testing.T) {
	fetch := func(string, *platform.Platform, *versions.Versions, map[string]bool) error {
		t.Error("fetch should not run")
		return nil
	}
	err := Run(filepath.Join(t.TempDir(), "out.tar.gz"), "mips", versions.Parse(""), Compiled{}, fetch)
	if err == nil {
		t.Fatal("expected error for garbage arch")
	}
}

func TestVerifyWrappersRejectsNonShebang(t *testing.T) {
	bin := t.TempDir()
	for _, name := range []string{"git", "zsh", "nvim", "go", "gofmt", "zig", "cc", "c++"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("not a wrapper\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := verifyWrappers(bin); err == nil {
		t.Fatal("expected error for non-shebang git")
	}
}

func oneFileTarGz(t *testing.T, dir, archiveName, rel, content string) string {
	t.Helper()
	src := t.TempDir()
	path := filepath.Join(src, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, archiveName)
	if err := archive.CreateTarGz(dest, src); err != nil {
		t.Fatalf("CreateTarGz: %v", err)
	}
	return dest
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
