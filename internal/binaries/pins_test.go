package binaries

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stefanpenner/devlayer/internal/sums"
	"github.com/stefanpenner/devlayer/internal/versions"
)

func TestSourceArchiveURLs(t *testing.T) {
	got := SourceArchiveURLs(testVers())
	want := map[string]string{
		"btop":    "https://github.com/aristocratos/btop/archive/refs/tags/v1.4.7.tar.gz",
		"htop":    "https://github.com/htop-dev/htop/releases/download/3.5.3/htop-3.5.3.tar.xz",
		"make":    "https://ftp.gnu.org/gnu/make/make-4.4.1.tar.gz",
		"ncurses": "https://ftp.gnu.org/gnu/ncurses/ncurses-6.6.tar.gz",
		"git":     "https://mirrors.edge.kernel.org/pub/software/scm/git/git-2.55.0.tar.xz",
		"zsh":                      "https://www.zsh.org/pub/zsh-5.9.2.tar.xz",
		"zlib":                     "https://zlib.net/fossils/zlib-1.3.1.tar.gz",
		"toybox":                   "https://github.com/landley/toybox/archive/refs/tags/0.8.14.tar.gz",
		"nvim":                     "https://github.com/neovim/neovim/archive/refs/tags/v0.12.5.tar.gz",
		"nvim_libuv":               "https://github.com/libuv/libuv/archive/v1.52.1.tar.gz",
		"nvim_luajit":              "https://github.com/luajit/luajit/archive/fbb36bb6bfa88716a47c58bcf9ce9f2ef752abac.tar.gz",
		"nvim_lua_compat53":        "https://github.com/lunarmodules/lua-compat-5.3/archive/v0.13.tar.gz",
		"nvim_luv":                 "https://github.com/luvit/luv/archive/1.52.1-0.tar.gz",
		"nvim_lpeg":                "https://github.com/neovim/deps/raw/d495ee6f79e7962a53ad79670cb92488abe0b9b4/opt/lpeg-1.1.0.tar.gz",
		"nvim_unibilium":           "https://github.com/neovim/unibilium/archive/v2.1.2.tar.gz",
		"nvim_utf8proc":            "https://github.com/juliastrings/utf8proc/archive/v2.11.3.tar.gz",
		"nvim_treesitter":          "https://github.com/tree-sitter/tree-sitter/archive/v0.26.13.tar.gz",
		"nvim_treesitter_c":        "https://github.com/tree-sitter/tree-sitter-c/archive/v0.24.1.tar.gz",
		"nvim_treesitter_lua":      "https://github.com/tree-sitter-grammars/tree-sitter-lua/archive/v0.5.0.tar.gz",
		"nvim_treesitter_vim":      "https://github.com/tree-sitter-grammars/tree-sitter-vim/archive/v0.8.1.tar.gz",
		"nvim_treesitter_vimdoc":   "https://github.com/neovim/tree-sitter-vimdoc/archive/v4.1.0.tar.gz",
		"nvim_treesitter_query":    "https://github.com/tree-sitter-grammars/tree-sitter-query/archive/v0.8.0.tar.gz",
		"nvim_treesitter_markdown": "https://github.com/tree-sitter-grammars/tree-sitter-markdown/archive/v0.5.3.tar.gz",
	}
	if len(got) != len(want) {
		t.Fatalf("len %d want %d (%v)", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s\n  got  %s\n  want %s", k, got[k], v)
		}
	}
}

func TestAllURLsPinned(t *testing.T) {
	vers := versions.Parse(string(readRepoFile(t, "versions.env")))
	urls, err := AllURLs(vers)
	if err != nil {
		t.Fatal(err)
	}
	pins, err := sums.Pins()
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]struct{}{}
	var missing []string
	for _, u := range urls {
		want[u] = struct{}{}
		if _, ok := pins[u]; !ok {
			missing = append(missing, u)
			fmt.Println("PINURL " + u)
		}
	}
	var extra []string
	for u := range pins {
		if _, ok := want[u]; !ok {
			extra = append(extra, u)
		}
	}
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("missing %d pins, extra %d\nextra:\n%s", len(missing), len(extra), strings.Join(extra, "\n"))
	}
}

func TestManifestTarget(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "MANIFEST")
	body := "_main/versions.env /repo/versions.env\n" +
		" _main/my\\sfile.env /repo/my\\bfile.env\n"
	if err := os.WriteFile(manifest, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := manifestTarget(manifest, "_main/versions.env"); !ok || got != filepath.FromSlash("/repo/versions.env") {
		t.Fatalf("plain got %q ok %v", got, ok)
	}
	if got, ok := manifestTarget(manifest, "_main/my file.env"); !ok || got != filepath.FromSlash("/repo/my\\file.env") {
		t.Fatalf("escaped got %q ok %v", got, ok)
	}
	if _, ok := manifestTarget(manifest, "_main/missing.env"); ok {
		t.Fatal("missing key")
	}
	if _, ok := manifestTarget("", "_main/versions.env"); ok {
		t.Fatal("empty manifest")
	}
}

func readRepoFile(t *testing.T, rel string) []byte {
	t.Helper()
	name := "_main/" + filepath.ToSlash(rel)
	if ws := os.Getenv("TEST_WORKSPACE"); ws != "" {
		name = ws + "/" + filepath.ToSlash(rel)
	}
	var cands []string
	// Windows Bazel tests ship a manifest, not a symlink tree.
	if p, ok := manifestTarget(os.Getenv("RUNFILES_MANIFEST_FILE"), name); ok {
		if b, err := os.ReadFile(p); err == nil {
			return b
		}
		cands = append(cands, p)
	}
	if src := os.Getenv("TEST_SRCDIR"); src != "" {
		cands = append(cands, filepath.Join(src, filepath.FromSlash(name)))
	}
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			cands = append(cands, filepath.Join(dir, rel))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	for _, p := range cands {
		b, err := os.ReadFile(p)
		if err == nil {
			return b
		}
	}
	t.Fatalf("could not read %s (tried %v)", rel, cands)
	return nil
}

// manifestTarget resolves one runfile from a Bazel manifest.
// A leading space means spaces and backslashes in the link are escaped.
func manifestTarget(manifest, name string) (string, bool) {
	if manifest == "" {
		return "", false
	}
	f, err := os.Open(manifest)
	if err != nil {
		return "", false
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		var link, target string
		if strings.HasPrefix(line, " ") {
			fields := strings.SplitN(line[1:], " ", 2)
			if len(fields) != 2 {
				continue
			}
			link = strings.NewReplacer(`\s`, " ", `\n`, "\n", `\b`, `\`).Replace(fields[0])
			target = strings.NewReplacer(`\n`, "\n", `\b`, `\`).Replace(fields[1])
		} else {
			fields := strings.SplitN(line, " ", 2)
			if len(fields) != 2 {
				continue
			}
			link, target = fields[0], fields[1]
		}
		if link == name {
			return filepath.FromSlash(target), true
		}
	}
	return "", false
}
