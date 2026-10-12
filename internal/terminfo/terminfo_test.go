package terminfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNames(t *testing.T) {
	got, err := Names("# comment\n\nxterm-256color\n\txterm\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"xterm-256color", "xterm"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("names = %q", got)
	}
}

func TestNamesRejects(t *testing.T) {
	cases := []string{
		"",
		"# only\n\n",
		"xterm\nxterm\n",
		"xterm extra\n",
		"../xterm\n",
		"xterm;rm\n",
		"has space\n",
		"xterm|ghostty\n",
		"\x00xterm\n",
		strings.Repeat("a", 80) + "\n",
	}
	for _, text := range cases {
		if _, err := Names(text); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
}

func TestNamesNeverPanics(t *testing.T) {
	payloads := []string{
		"",
		"\n",
		"\x00",
		strings.Repeat("x", 1<<20),
		"\r\n\r\n",
		"#\n#\n",
		"ok\n",
		"dup\ndup\n",
		"a b\n",
		"../../etc/passwd\n",
		"xterm-256color\x00hidden\n",
	}
	for _, text := range payloads {
		func() {
			defer func() {
				if recover() != nil {
					t.Fatalf("panic on %q", text[:min(40, len(text))])
				}
			}()
			_, _ = Names(text)
		}()
	}
}

func FuzzNames(f *testing.F) {
	f.Add("xterm-256color\n# c\n\nxterm\n")
	f.Add("")
	f.Add("dup\ndup\n")
	f.Add("bad name\n")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = Names(s)
	})
}

func TestStaticOptions(t *testing.T) {
	got := StaticOptions([]string{"xterm", "dumb"}, "/tic", "/infocmp")
	text := strings.Join(got, "\n")
	for _, want := range []string{
		"--disable-database",
		"--disable-db-install",
		"--with-fallbacks=xterm,dumb",
		"--with-tic-path=/tic",
		"--with-infocmp-path=/infocmp",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in %q", want, got)
		}
	}
	if strings.Contains(text, "--without-database") {
		t.Fatalf("without-database does not disable the search: %q", got)
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "xterm dumb", "#define NCURSES_USE_DATABASE 0\n")
	if err := Check(dir, []string{"xterm", "dumb"}); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir, []string{"dumb", "xterm"}); err == nil {
		t.Fatal("order mismatch accepted")
	}
	writeFixture(t, dir, "xterm dumb", "#define NCURSES_USE_DATABASE 1\n")
	if err := Check(dir, []string{"xterm", "dumb"}); err == nil {
		t.Fatal("database on accepted")
	}
	writeFixture(t, dir, "xterm dumb", "#define NCURSES_USE_DATABASE @NCURSES_USE_DATABASE@\n")
	if err := Check(dir, []string{"xterm", "dumb"}); err == nil {
		t.Fatal("unsubstituted define accepted")
	}
}

func TestRepoList(t *testing.T) {
	text := repoFile(t, "third_party/ncurses/fallbacks.txt")
	names, err := Names(text)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, name := range names {
		got[name] = true
	}
	for _, name := range []string{
		"xterm-256color", "xterm", "tmux-256color", "screen-256color",
		"linux", "dumb", "ghostty", "xterm-ghostty", "alacritty", "kitty",
	} {
		if !got[name] {
			t.Fatalf("list omits %s", name)
		}
	}
	aliases := repoFile(t, "third_party/ncurses/aliases.ti")
	if !strings.Contains(aliases, "xterm-ghostty|") || !strings.Contains(aliases, "use=ghostty") {
		t.Fatalf("aliases.ti = %q", aliases)
	}
	script := repoFile(t, "third_party/ncurses/configure.sh")
	for _, want := range []string{
		"--disable-database", "--with-fallbacks", "fallbacks.txt",
		"AR=/usr/bin/ar", "RANLIB=/usr/bin/ranlib", "cf_cv_ar_flags=cr",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("configure.sh omits %s", want)
		}
	}
	if strings.Contains(script, "--without-database") {
		t.Fatal("configure.sh uses --without-database")
	}
	comma := scriptList(t, repoPath(t, "third_party/ncurses/fallbacks.txt"))
	if comma != strings.Join(names, ",") {
		t.Fatalf("script list = %s", comma)
	}
}

func TestScriptAppendAndCheck(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(src, "misc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "misc", "terminfo.src"), []byte("dumb|dumb,\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	aliases := filepath.Join(dir, "aliases.ti")
	if err := os.WriteFile(aliases, []byte("xterm-ghostty|Ghostty terminal,\n\tuse=ghostty,\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := repoPath(t, "third_party/ncurses/configure.sh")
	runCfg(t, script, "append", src, aliases)
	runCfg(t, script, "append", src, aliases)
	body, err := os.ReadFile(filepath.Join(src, "misc", "terminfo.src"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(body), "# devlayer:aliases") != 1 {
		t.Fatalf("append is not idempotent: %s", body)
	}
	if !strings.Contains(string(body), "use=ghostty") {
		t.Fatalf("alias missing: %s", body)
	}

	list := filepath.Join(dir, "fallbacks.txt")
	if err := os.WriteFile(list, []byte("xterm\ndumb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, src, "xterm dumb", "#define NCURSES_USE_DATABASE 0\n")
	runCfg(t, script, "check", src, list)
	writeFixture(t, src, "dumb", "#define NCURSES_USE_DATABASE 0\n")
	cmd := exec.Command(script, "check", src, list)
	if err := cmd.Run(); err == nil {
		t.Fatal("check accepted a short list")
	}
}

func TestBuildsAreSuperstatic(t *testing.T) {
	build := repoFile(t, "third_party/ncurses/BUILD.bazel")
	for _, want := range []string{"configure.sh", "fallbacks.txt", "configure_in_place", "-static -o"} {
		if !strings.Contains(build, want) {
			t.Fatalf("ncurses BUILD omits %s", want)
		}
	}
	if strings.Contains(build, "--without-database") {
		t.Fatal("ncurses BUILD uses --without-database")
	}
	for _, rel := range []string{
		"cmd/build.go",
		"internal/sysroot/build.go",
		"internal/sysroot/pins.go",
	} {
		text := repoFile(t, rel)
		if strings.Contains(text, "--without-database") || strings.Contains(text, "--with-fallbacks=") {
			t.Fatalf("%s hardcodes a terminfo flag", rel)
		}
	}
	if !strings.Contains(repoFile(t, "internal/sysroot/pins.go"), "ncurses-6.6.tar.gz") {
		t.Fatal("sysroot ncurses is not the 6.6 pin")
	}
	mac := repoFile(t, "macos/defs.bzl")
	if strings.Contains(mac, "/usr/share/terminfo") {
		t.Fatal("macos defs compiles a host terminfo path")
	}
	if !strings.Contains(mac, "unibilium_no_host.patch") {
		t.Fatal("macos defs omits the unibilium patch")
	}
	archive := repoFile(t, "macos/archive.sh")
	if !strings.Contains(archive, "/usr/bin/patch") {
		t.Fatal("archive.sh does not apply a patch")
	}
	ext := repoFile(t, "tools/extensions.bzl")
	nvim := repoFile(t, "third_party/nvim/defs.bzl")
	if !strings.Contains(nvim, "DEVLAYER_UNIBI_PATCH") {
		t.Fatal("nvim deps do not receive the unibilium patch")
	}
	if !strings.Contains(ext, "nvim_unibilium_superstatic.patch") {
		t.Fatal("nvim source does not apply the cmake patch")
	}
	cmakePatch := repoFile(t, "third_party/patches/nvim_unibilium_superstatic.patch")
	if !strings.Contains(cmakePatch, "-DTERMINFO_DIRS=") || !strings.Contains(cmakePatch, "DEVLAYER_UNIBI_PATCH") {
		t.Fatal("nvim cmake patch does not disable host terminfo")
	}
	uni := repoFile(t, "third_party/patches/unibilium_no_host.patch")
	if !strings.Contains(uni, "-    if ((env = getenv(\"TERMINFO\")))") {
		t.Fatal("unibilium patch does not remove the host search")
	}
	if !strings.Contains(uni, "set(TERMINFO_DIRS \"\"") {
		t.Fatal("unibilium patch does not clear TERMINFO_DIRS")
	}
	policy := repoFile(t, "HERMETICITY.md")
	if !strings.Contains(policy, "## Superstatic") || !strings.Contains(policy, "fallbacks.txt") {
		t.Fatal("HERMETICITY.md has no superstatic rule")
	}
}

func writeFixture(t *testing.T, dir, list, define string) {
	t.Helper()
	mk := "FALLBACK_LIST\t= " + list + "\n"
	if err := os.MkdirAll(filepath.Join(dir, "ncurses"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "include"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ncurses", "Makefile"), []byte(mk), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "include", "term.h"), []byte(define), 0o644); err != nil {
		t.Fatal(err)
	}
	// The upstream template keeps the token. The check reads the generated file.
	template := "#define NCURSES_USE_DATABASE @NCURSES_USE_DATABASE@\n"
	if err := os.WriteFile(filepath.Join(dir, "include", "MKterm.h.awk.in"), []byte(template), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scriptList(t *testing.T, file string) string {
	t.Helper()
	script := repoPath(t, "third_party/ncurses/configure.sh")
	out, err := exec.Command(script, "list", file).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func runCfg(t *testing.T, script string, args ...string) {
	t.Helper()
	cmd := exec.Command(script, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %s", err, out)
	}
}

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(repoPath(t, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func repoPath(t *testing.T, rel string) string {
	t.Helper()
	rel = filepath.FromSlash(rel)
	ws := os.Getenv("TEST_WORKSPACE")
	if ws == "" {
		ws = "_main"
	}
	var cands []string
	if src := os.Getenv("TEST_SRCDIR"); src != "" {
		cands = append(cands, filepath.Join(src, ws, rel))
	}
	if wd, err := os.Getwd(); err == nil {
		cands = append(cands, filepath.Join(wd, rel))
	}
	_, here, _, ok := runtime.Caller(0)
	if ok {
		cands = append(cands, filepath.Join(filepath.Dir(here), "..", "..", rel))
	}
	for _, path := range cands {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	t.Fatalf("missing %s", rel)
	return ""
}
