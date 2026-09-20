package assemble

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stefanpenner/devlayer/internal/archive"
	"github.com/stefanpenner/devlayer/internal/platform"
	"github.com/stefanpenner/devlayer/internal/versions"
)

func TestWrappers(t *testing.T) {
	prefix := `PREFIX="$(cd "$(dirname "$0")/.." && pwd)"`
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
			body: nvimWrapper(),
			want: []string{
				"#!/bin/sh",
				prefix,
				"VIMRUNTIME=",
				"nvim/share/nvim/runtime",
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
