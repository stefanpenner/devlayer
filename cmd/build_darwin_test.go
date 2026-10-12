package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDarwinCCScriptUsesZigAndSDK(t *testing.T) {
	got := darwinCCScript("/pin/zig", "/pin/MacOSX.sdk", "aarch64", "cc", "")
	for _, part := range []string{
		"/pin/zig",
		"cc",
		"-target aarch64-macos",
		"--sysroot \"/pin/MacOSX.sdk\"",
		"-isystem \"/pin/MacOSX.sdk/usr/include\"",
		"-iframework \"/pin/MacOSX.sdk/System/Library/Frameworks\"",
		"-F \"/pin/MacOSX.sdk/System/Library/Frameworks\"",
		"-mmacosx-version-min=11.0",
		"-fno-sanitize=undefined",
		"DEVELOPER_DIR=/tmp/devlayer-no-xcrun",
	} {
		if !strings.Contains(got, part) {
			t.Fatalf("script missing %q:\n%s", part, got)
		}
	}
	if strings.Contains(got, "-L/usr/lib") {
		t.Fatalf("script searches the SDK lib dir:\n%s", got)
	}
	if strings.Contains(got, "/usr/bin/xcrun") || strings.Contains(got, "\nxcrun ") {
		t.Fatalf("script calls xcrun:\n%s", got)
	}
}

func TestDarwinCCScriptRewritesBundle(t *testing.T) {
	dir := t.TempDir()
	zig := filepath.Join(dir, "zig")
	argsPath := filepath.Join(dir, "args")
	zigBody := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsPath + "\n"
	if err := os.WriteFile(zig, []byte(zigBody), 0o755); err != nil {
		t.Fatal(err)
	}
	cc := filepath.Join(dir, "cc")
	if err := os.WriteFile(cc, []byte(darwinCCScript(zig, "/pin/MacOSX.sdk", "aarch64", "cc", "")), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(cc, "-O3", "-bundle", "-Wl,-export_dynamic", "--target=arm64-apple-macosx", "-o", "query.so", "parser.o")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if strings.Contains(text, "-bundle\n") || strings.HasSuffix(strings.TrimSpace(text), "-bundle") {
		t.Fatalf("script kept -bundle:\n%s", text)
	}
	if !strings.Contains(text, "-dynamiclib") {
		t.Fatalf("script dropped the bundle link:\n%s", text)
	}
	if !strings.Contains(text, "-undefined,dynamic_lookup") {
		t.Fatalf("script dropped dynamic_lookup:\n%s", text)
	}
	if strings.Contains(text, "-export_dynamic") {
		t.Fatalf("script kept -export_dynamic:\n%s", text)
	}
	if !strings.Contains(text, "-rdynamic") {
		t.Fatalf("script dropped -rdynamic:\n%s", text)
	}
	if strings.Contains(text, "arm64-apple-macosx") {
		t.Fatalf("script kept the Apple target:\n%s", text)
	}
	if !strings.Contains(text, "--target=aarch64-macos") {
		t.Fatalf("script dropped the Zig target:\n%s", text)
	}
}

func TestDarwinCCScriptUsesStaticArchives(t *testing.T) {
	dir := t.TempDir()
	static := filepath.Join(dir, "static")
	iconvLib := filepath.Join(static, "iconv", "lib")
	cursesLib := filepath.Join(static, "ncurses", "lib")
	if err := os.MkdirAll(iconvLib, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cursesLib, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"libiconv.a", "libcharset.a"} {
		if err := os.WriteFile(filepath.Join(iconvLib, name), []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(cursesLib, "libncursesw.a"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	zig := filepath.Join(dir, "zig")
	argsPath := filepath.Join(dir, "args")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsPath + "\n"
	if err := os.WriteFile(zig, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	cc := filepath.Join(dir, "cc")
	script := darwinCCScript(zig, "/pin/MacOSX.sdk", "aarch64", "cc", static)
	if err := os.WriteFile(cc, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cc, "-liconv", "-lncursesw", "-o", "out", "in.c")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	scriptText := script
	execAt := strings.LastIndex(scriptText, "\nexec ")
	if execAt < 0 {
		t.Fatalf("script has no exec:\n%s", scriptText)
	}
	execLine := scriptText[execAt:]
	incAt := strings.Index(execLine, "-I \"")
	sysAt := strings.Index(execLine, "-isystem ")
	if incAt < 0 || sysAt < 0 || incAt > sysAt {
		t.Fatalf("static include is not before the SDK include:\n%s", execLine)
	}
	lines := strings.Split(text, "\n")
	iAt, sysLine := -1, -1
	for i, line := range lines {
		if line == "-I" && iAt < 0 {
			iAt = i
		}
		if line == "-isystem" && sysLine < 0 {
			sysLine = i
		}
	}
	incDir := filepath.Join(static, "iconv", "include")
	if iAt < 0 || sysLine < 0 || iAt > sysLine || iAt+1 >= len(lines) || lines[iAt+1] != incDir {
		t.Fatalf("argv include order:\n%s", text)
	}
	for _, part := range []string{
		filepath.Join(iconvLib, "libiconv.a"),
		filepath.Join(cursesLib, "libncursesw.a"),
	} {
		if !strings.Contains(text, part) {
			t.Fatalf("script missed %s:\n%s", part, text)
		}
	}
	if strings.Contains(text, filepath.Join(iconvLib, "libcharset.a")) {
		t.Fatalf("script linked libcharset.a:\n%s", text)
	}
	for _, banned := range []string{"-liconv\n", "-lncursesw\n"} {
		if strings.Contains(text, banned) {
			t.Fatalf("script kept %q:\n%s", strings.TrimSpace(banned), text)
		}
	}
}

func TestNvimMakeArgsDropHostLibs(t *testing.T) {
	got := strings.Join(nvimMakeArgs("/out", "/static"), "\n")
	for _, part := range []string{
		"-DENABLE_LIBINTL=OFF",
		"-DCMAKE_IGNORE_PREFIX_PATH=/opt/homebrew",
		"-DICONV_LIBRARY=/static/iconv/lib/libiconv.a",
		"-DICONV_INCLUDE_DIR=/static/iconv/include",
	} {
		if !strings.Contains(got, part) {
			t.Fatalf("missing %s in\n%s", part, got)
		}
	}
}

func TestHtopConfigureUsesStaticCurses(t *testing.T) {
	got := strings.Join(htopConfigureArgs("/static"), "\n")
	for _, part := range []string{
		"CURSES_LIBS=/static/ncurses/lib/libncursesw.a",
		"CURSES_CFLAGS=-I/static/ncurses/include/ncursesw -I/static/ncurses/include",
	} {
		if !strings.Contains(got, part) {
			t.Fatalf("missing %s in\n%s", part, got)
		}
	}
}

func TestDarwinCCScriptRejectsUnknownLib(t *testing.T) {
	dir := t.TempDir()
	zig := filepath.Join(dir, "zig")
	if err := os.WriteFile(zig, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cc := filepath.Join(dir, "cc")
	if err := os.WriteFile(cc, []byte(darwinCCScript(zig, "/pin/MacOSX.sdk", "aarch64", "cc", "")), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cc, "-lfoo")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("accepted -lfoo:\n%s", out)
	}
	if !strings.Contains(string(out), "refusing -lfoo") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestDarwinCCScriptDropsSystemIconvInclude(t *testing.T) {
	dir := t.TempDir()
	static := filepath.Join(dir, "static")
	inc := filepath.Join(static, "iconv", "include")
	zig := filepath.Join(dir, "zig")
	argsPath := filepath.Join(dir, "args")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsPath + "\n"
	if err := os.WriteFile(zig, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	cc := filepath.Join(dir, "cc")
	script := darwinCCScript(zig, "/pin/MacOSX.sdk", "aarch64", "cc", static)
	if err := os.WriteFile(cc, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cc, "-isystem", inc, "-c", "in.c")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line == "-isystem" && i+1 < len(lines) && lines[i+1] == inc {
			t.Fatalf("kept system iconv include:\n%s", text)
		}
	}
	saw := false
	for i, line := range lines {
		if line == "-I" && i+1 < len(lines) && lines[i+1] == inc {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("missing plain iconv include:\n%s", text)
	}
}

func TestDarwinCCScriptMapsLibutilToStub(t *testing.T) {
	dir := t.TempDir()
	zig := filepath.Join(dir, "zig")
	argsPath := filepath.Join(dir, "args")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsPath + "\n"
	if err := os.WriteFile(zig, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	cc := filepath.Join(dir, "cc")
	if err := os.WriteFile(cc, []byte(darwinCCScript(zig, "/pin/MacOSX.sdk", "aarch64", "cc", "")), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cc, "-lutil", "-lm")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "/pin/MacOSX.sdk/usr/lib/libutil.tbd\n") {
		t.Fatalf("missed libutil stub:\n%s", text)
	}
	if !strings.Contains(text, "-lm\n") {
		t.Fatalf("dropped -lm:\n%s", text)
	}
	if strings.Contains(text, "-lutil\n") {
		t.Fatalf("kept -lutil:\n%s", text)
	}
}

func TestDarwinCCScriptKeepsOutputInstallName(t *testing.T) {
	dir := t.TempDir()
	zig := filepath.Join(dir, "zig")
	argsPath := filepath.Join(dir, "args")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsPath + "\n"
	if err := os.WriteFile(zig, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	cc := filepath.Join(dir, "cc")
	if err := os.WriteFile(cc, []byte(darwinCCScript(zig, "/pin/MacOSX.sdk", "aarch64", "cc", "")), 0o755); err != nil {
		t.Fatal(err)
	}
	installName := filepath.Join(dir, "libluajit-5.1.2.dylib")
	cmd := exec.Command(cc, "-dynamiclib", "-install_name", installName, "-o", "libluajit.so")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), installName) {
		t.Fatalf("dropped install name:\n%s", got)
	}
}

func TestDarwinCCScriptAllowsRustRlib(t *testing.T) {
	dir := t.TempDir()
	zig := filepath.Join(dir, "zig")
	argsPath := filepath.Join(dir, "args")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsPath + "\n"
	if err := os.WriteFile(zig, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	cc := filepath.Join(dir, "cc")
	if err := os.WriteFile(cc, []byte(darwinCCScript(zig, "/pin/MacOSX.sdk", "aarch64", "cc", "")), 0o755); err != nil {
		t.Fatal(err)
	}
	rlib := "/opt/homebrew/Cellar/rust/1.98.0/lib/rustlib/aarch64-apple-darwin/lib/libstd.rlib"
	cmd := exec.Command(cc, rlib, "-o", "out")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), rlib+"\n") {
		t.Fatalf("dropped rust rlib:\n%s", got)
	}
}

func TestDarwinCCScriptRejectsHomebrewDylib(t *testing.T) {
	dir := t.TempDir()
	zig := filepath.Join(dir, "zig")
	if err := os.WriteFile(zig, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cc := filepath.Join(dir, "cc")
	if err := os.WriteFile(cc, []byte(darwinCCScript(zig, "/pin/MacOSX.sdk", "aarch64", "cc", "")), 0o755); err != nil {
		t.Fatal(err)
	}
	dylib := "/opt/homebrew/lib/libgit2.dylib"
	cmd := exec.Command(cc, dylib)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("accepted homebrew dylib:\n%s", out)
	}
	if !strings.Contains(string(out), "refusing "+dylib) {
		t.Fatalf("output:\n%s", out)
	}
}

func TestDarwinCCScriptRejectsHostDylib(t *testing.T) {
	dir := t.TempDir()
	zig := filepath.Join(dir, "zig")
	if err := os.WriteFile(zig, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cc := filepath.Join(dir, "cc")
	if err := os.WriteFile(cc, []byte(darwinCCScript(zig, "/pin/MacOSX.sdk", "aarch64", "cc", "")), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cc, "/usr/lib/libiconv.2.dylib")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("accepted host dylib:\n%s", out)
	}
	if !strings.Contains(string(out), "refusing /usr/lib/libiconv.2.dylib") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestDarwinCCScriptUsesStaticLibFromLaterSearchDir(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "zlib")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(lib, "libz.a")
	if err := os.WriteFile(archive, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	zig := filepath.Join(dir, "zig")
	argsPath := filepath.Join(dir, "args")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsPath + "\n"
	if err := os.WriteFile(zig, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	cc := filepath.Join(dir, "cc")
	if err := os.WriteFile(cc, []byte(darwinCCScript(zig, "/pin/MacOSX.sdk", "aarch64", "cc", "")), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cc, "-lz", "-L", lib, "-o", "out")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, archive) {
		t.Fatalf("missed static zlib:\n%s", text)
	}
	if strings.Contains(text, "-lz\n") || strings.Contains(text, "-L\n") {
		t.Fatalf("kept a library search:\n%s", text)
	}
}

func TestDarwinCCScriptKeepsSystemLibFlags(t *testing.T) {
	dir := t.TempDir()
	zig := filepath.Join(dir, "zig")
	argsPath := filepath.Join(dir, "args")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsPath + "\n"
	if err := os.WriteFile(zig, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	cc := filepath.Join(dir, "cc")
	script := darwinCCScript(zig, "/pin/MacOSX.sdk", "aarch64", "cc", "")
	if err := os.WriteFile(cc, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cc, "-lSystem", "-lc++")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, part := range []string{"-lSystem\n", "-lc++\n"} {
		if !strings.Contains(text, part) {
			t.Fatalf("missed %q:\n%s", strings.TrimSpace(part), text)
		}
	}
	if strings.Contains(text, ".tbd") {
		t.Fatalf("system lib became a stub file:\n%s", text)
	}
}

func TestConfigLogKeepsTheCompilerSection(t *testing.T) {
	dir := t.TempDir()
	var body strings.Builder
	body.WriteString("configure:5360: checking whether the C compiler works\n")
	body.WriteString("configure:5382: /tmp/cc conftest.c\n")
	body.WriteString("zig: error: unable to find libSystem\n")
	body.WriteString("configure:5386: $? = 1\n")
	for i := 0; i < 120; i++ {
		fmt.Fprintf(&body, "cache-%03d=''\n", i)
	}
	body.WriteString("configure: exit 77\n")
	if err := os.WriteFile(filepath.Join(dir, "config.log"), []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	got := configLogTail(dir)
	if !strings.Contains(got, "unable to find libSystem") {
		t.Fatalf("section dropped the compiler error:\n%s", got)
	}
	if strings.Contains(got, "cache-119") {
		t.Fatalf("section kept the cache dump:\n%s", got)
	}
}

func TestConfigLogTailKeepsTheCompilerError(t *testing.T) {
	dir := t.TempDir()
	var body strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&body, "line-%03d\n", i)
	}
	body.WriteString("zig: error: the sdk has no libSystem\n")
	if err := os.WriteFile(filepath.Join(dir, "config.log"), []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	got := configLogTail(dir)
	if !strings.Contains(got, "the sdk has no libSystem") {
		t.Fatalf("tail dropped the compiler error:\n%s", got)
	}
	if strings.Contains(got, "line-000") {
		t.Fatalf("tail kept the log head:\n%s", got)
	}
	if !strings.Contains(got, "line-199") {
		t.Fatalf("tail dropped the lines near the error:\n%s", got)
	}
}

func TestResolveBazelFileFindsExternalUnderOutputBase(t *testing.T) {
	root := t.TempDir()
	execRoot := filepath.Join(root, "exec")
	outputBase := filepath.Join(root, "out")
	scriptDir := filepath.Join(root, "src")
	rel := filepath.Join("external", "zig_config", "zig")
	full := filepath.Join(outputBase, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("zig"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := resolveBazelFile(rel, scriptDir, execRoot, outputBase)
	if err != nil {
		t.Fatal(err)
	}
	if got != full {
		t.Fatalf("got %s", got)
	}
}

func TestResolveBazelFileKeepsExternalOutOfExecRoot(t *testing.T) {
	root := t.TempDir()
	execRoot := filepath.Join(root, "exec")
	outputBase := filepath.Join(root, "out")
	scriptDir := filepath.Join(root, "src")
	rel := filepath.Join("external", "zig_config", "zig")
	for _, base := range []string{execRoot, outputBase} {
		full := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(base), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := resolveBazelFile(rel, scriptDir, execRoot, outputBase)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(outputBase, rel)
	if got != want {
		t.Fatalf("got %s", got)
	}
}

func TestResolveBazelFilePrefersExecRootForBuildOutput(t *testing.T) {
	root := t.TempDir()
	execRoot := filepath.Join(root, "exec")
	outputBase := filepath.Join(root, "out")
	scriptDir := filepath.Join(root, "src")
	rel := filepath.Join("bazel-out", "fastbuild", "bin", "sdk.tar")
	full := filepath.Join(execRoot, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("tar"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveBazelFile(rel, scriptDir, execRoot, outputBase)
	if err != nil {
		t.Fatal(err)
	}
	if got != full {
		t.Fatalf("got %s", got)
	}
}

func TestFinishBundleRejectsOutsideSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "host")
	if err := os.WriteFile(outside, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "delta")); err != nil {
		t.Fatal(err)
	}
	err := finishBundle(root, filepath.Join(t.TempDir(), "out.tar.gz"), "tar.gz")
	if err == nil || !strings.Contains(err.Error(), "leaves the tree") {
		t.Fatalf("got %v", err)
	}
}
