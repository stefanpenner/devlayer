package loadcheck

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxDevlayerPolicyIsStaticElf(t *testing.T) {
	path, err := policyPath("bininfo/policy/linux/bin/devlayer.toml")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !bytes.Contains(body, []byte("format = \"elf\"")) || !bytes.Contains(body, []byte("DT_NEEDED = []")) {
		t.Fatalf("linux CLI policy is not a static ELF:\n%s", text)
	}
	if bytes.Contains(body, []byte("libc.so")) || bytes.Contains(body, []byte("PT_INTERP = \"/")) {
		t.Fatalf("linux CLI policy names a host library:\n%s", text)
	}
}

func policyPath(rel string) (string, error) {
	if _, err := os.Stat(rel); err == nil {
		return rel, nil
	}
	root := os.Getenv("TEST_SRCDIR")
	if root == "" {
		return "", os.ErrNotExist
	}
	ws := os.Getenv("TEST_WORKSPACE")
	if ws == "" {
		ws = "_main"
	}
	for _, cand := range []string{filepath.Join(root, ws, rel), filepath.Join(root, rel)} {
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}
	return "", os.ErrNotExist
}

func TestDevlayerBinaryMatchesPolicy(t *testing.T) {
	bin, err := resolve("DEVLAYER_BIN")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "bin", "devlayer")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, body, 0o755); err != nil {
		t.Fatal(err)
	}
	if len(body) >= 4 && string(body[:4]) == "\x7fELF" {
		policy, err := policyPath("bininfo/policy/linux")
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("BININFO_POLICY", policy)
	}
	if err := File(dst); err != nil {
		t.Fatal(err)
	}
}

func TestTreeSkipsThePinnedGoSDK(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "go", "pkg", "tool", "link.exe")
	if err := os.MkdirAll(filepath.Dir(tool), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tool, IconvELF(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Tree(dir); err != nil {
		t.Fatal(err)
	}
}

func TestMinGitPoliciesRecordTheNamedFiles(t *testing.T) {
	for _, rel := range []string{
		"git/usr/lib/ssh/ssh-pkcs11-helper.exe",
		"git/usr/lib/ssh/ssh-sk-helper.exe",
		"git/usr/lib/ssh/ssh-keysign.exe",
		"git/usr/libexec/getprocaddr64.exe",
		"git/usr/libexec/getprocaddr32.exe",
		"git/usr/bin/ssh-add.exe",
		"git/usr/bin/basename.exe",
		"git/usr/bin/cygwin-console-helper.exe",
		"git/usr/bin/wc.exe",
		"git/usr/bin/cut.exe",
		"git/usr/bin/rebase.exe",
		"git/usr/bin/msys-intl-8.dll",
		"git/usr/bin/tr.exe",
		"git/usr/bin/diff.exe",
		"git/usr/bin/grep.exe",
		"git/usr/bin/msys-ncursesw6.dll",
		"git/usr/bin/msys-com_err-1.dll",
		"git/usr/bin/msys-gmp-10.dll",
		"git/usr/bin/newgrp.exe",
		"git/usr/bin/basenc.exe",
		"git/usr/bin/cp.exe",
		"git/usr/bin/sdiff.exe",
		"git/usr/bin/msys-krb5-26.dll",
		"git/usr/bin/env.exe",
		"git/usr/bin/pwd.exe",
		"git/usr/bin/tail.exe",
		"git/usr/bin/readlink.exe",
		"git/usr/bin/ssh.exe",
		"git/usr/bin/msys-wind-0.dll",
		"git/usr/bin/sh.exe",
		"git/usr/bin/msys-sqlite3-0.dll",
		"git/usr/bin/sort.exe",
	} {
		path, err := policyPath("bininfo/policy/" + rel + ".toml")
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte("format = \"pe\"")) || !bytes.Contains(body, []byte("IMAGE_IMPORT_DESCRIPTOR")) {
			t.Fatalf("%s is not a PE policy:\n%s", rel, body)
		}
		if !bytes.Contains(body, []byte("why = ")) {
			t.Fatalf("%s has no why:\n%s", rel, body)
		}
	}
}

func TestWindowsPoliciesRecordTheNamedFiles(t *testing.T) {
	for _, rel := range []string{
		"nvim/lib/nvim/parser/query.dll",
		"nvim/lib/nvim/parser/lua.dll",
		"nvim/lib/nvim/parser/vim.dll",
		"nvim/lib/nvim/parser/markdown.dll",
		"nvim/lib/nvim/parser/vimdoc.dll",
		"nvim/lib/nvim/parser/c.dll",
		"nvim/lib/nvim/parser/markdown_inline.dll",
		"nvim/bin/nvim.exe",
		"nvim/bin/lua51.dll",
		"nvim/bin/tee.exe",
		"nvim/bin/DbgHelp.dll",
		"nvim/bin/xxd.exe",
		"nvim/bin/win32yank.exe",
		"zig/zig.exe",
		"bin/delta.exe",
		"bin/glow.exe",
		"bin/bat.exe",
		"bin/jq.exe",
		"bin/shh.exe",
		"bin/age-keygen.exe",
		"bin/eza.exe",
		"bin/fd.exe",
		"bin/lazygit.exe",
		"bin/direnv.exe",
		"bin/dust.exe",
		"bin/fzf.exe",
		"bin/rg.exe",
		"bin/age.exe",
	} {
		path, err := policyPath("bininfo/policy/" + rel + ".toml")
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte("format = \"pe\"")) || !bytes.Contains(body, []byte("IMAGE_IMPORT_DESCRIPTOR")) {
			t.Fatalf("%s is not a PE policy:\n%s", rel, body)
		}
		if !bytes.Contains(body, []byte("why = ")) {
			t.Fatalf("%s has no why:\n%s", rel, body)
		}
		base := []byte("IMAGE_DLLCHARACTERISTICS_DYNAMIC_BASE = true")
		if rel == "bin/eza.exe" {
			base = []byte("IMAGE_DLLCHARACTERISTICS_DYNAMIC_BASE = false")
		}
		if !bytes.Contains(body, base) {
			t.Fatalf("%s dynamic base:\n%s", rel, body)
		}
	}
}

func TestDarwinNvimPoliciesRecordTheNamedFiles(t *testing.T) {
	for _, rel := range []string{
		"bin/nvim",
		"lib/nvim/parser/c.so",
		"lib/nvim/parser/vim.so",
		"lib/nvim/parser/markdown.so",
		"lib/nvim/parser/vimdoc.so",
		"lib/nvim/parser/lua.so",
		"lib/nvim/parser/query.so",
		"lib/nvim/parser/markdown_inline.so",
	} {
		path, err := policyPath("bininfo/policy/" + rel + ".toml")
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte("format = \"macho\"")) || !bytes.Contains(body, []byte("LC_LOAD_DYLIB")) {
			t.Fatalf("%s is not a Mach-O policy:\n%s", rel, body)
		}
		if !bytes.Contains(body, []byte("why = ")) {
			t.Fatalf("%s has no why:\n%s", rel, body)
		}
		if bytes.Contains(body, []byte("/opt/homebrew")) || bytes.Contains(body, []byte("/usr/local/")) {
			t.Fatalf("%s names a Homebrew library:\n%s", rel, body)
		}
		if rel == "bin/nvim" {
			if !bytes.Contains(body, []byte("CoreServices")) || !bytes.Contains(body, []byte("libSystem.B.dylib")) || !bytes.Contains(body, []byte("libobjc.A.dylib")) {
				t.Fatalf("%s dylibs:\n%s", rel, body)
			}
			continue
		}
		if bytes.Contains(body, []byte("CoreServices")) || !bytes.Contains(body, []byte("libSystem.B.dylib")) {
			t.Fatalf("%s dylibs:\n%s", rel, body)
		}
	}
}

func TestDarwinHtopPolicyRecordsTheNamedFile(t *testing.T) {
	body := readPolicy(t, "bininfo/policy/bin/htop.toml")
	if !bytes.Contains(body, []byte("format = \"macho\"")) || !bytes.Contains(body, []byte("filetype = \"MH_EXECUTE\"")) || !bytes.Contains(body, []byte("MH_PIE = true")) {
		t.Fatalf("htop policy:\n%s", body)
	}
	if bytes.Contains(body, []byte("/opt/homebrew")) || bytes.Contains(body, []byte("/usr/local/")) {
		t.Fatalf("htop names a Homebrew library:\n%s", body)
	}
	order := []string{
		"/System/Library/Frameworks/IOKit.framework/Versions/A/IOKit",
		"/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation",
		"/usr/lib/libSystem.B.dylib",
		"/usr/lib/libobjc.A.dylib",
	}
	at := 0
	for _, name := range order {
		needle := []byte("name = \"" + name + "\"")
		i := bytes.Index(body[at:], needle)
		if i < 0 {
			t.Fatalf("htop needed %s:\n%s", name, body)
		}
		at += i + len(needle)
	}
	if bytes.Count(body, []byte("why = ")) < len(order) {
		t.Fatalf("htop why:\n%s", body)
	}
}

func TestDarwinBtopPolicyRecordsTheNamedFile(t *testing.T) {
	body := readPolicy(t, "bininfo/policy/bin/btop.toml")
	if !bytes.Contains(body, []byte("format = \"macho\"")) || !bytes.Contains(body, []byte("filetype = \"MH_EXECUTE\"")) || !bytes.Contains(body, []byte("MH_PIE = true")) {
		t.Fatalf("btop policy:\n%s", body)
	}
	if !bytes.Contains(body, []byte("MH_WEAK_DEFINES")) || !bytes.Contains(body, []byte("MH_BINDS_TO_WEAK")) || !bytes.Contains(body, []byte("MH_HAS_TLV_DESCRIPTORS")) {
		t.Fatalf("btop flags:\n%s", body)
	}
	if bytes.Contains(body, []byte("/opt/homebrew")) || bytes.Contains(body, []byte("/usr/local/")) {
		t.Fatalf("btop names a Homebrew library:\n%s", body)
	}
	order := []string{
		"/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation",
		"/System/Library/Frameworks/IOKit.framework/Versions/A/IOKit",
		"/usr/lib/libSystem.B.dylib",
		"/usr/lib/libobjc.A.dylib",
	}
	at := 0
	for _, name := range order {
		needle := []byte("name = \"" + name + "\"")
		i := bytes.Index(body[at:], needle)
		if i < 0 {
			t.Fatalf("btop needed %s:\n%s", name, body)
		}
		at += i + len(needle)
	}
	if bytes.Count(body, []byte("why = ")) < len(order) {
		t.Fatalf("btop why:\n%s", body)
	}
}

func TestDarwinEzaPolicyRecordsTheNamedFile(t *testing.T) {
	body := readPolicy(t, "bininfo/policy/libexec/eza.toml")
	if !bytes.Contains(body, []byte("format = \"macho\"")) || !bytes.Contains(body, []byte("filetype = \"MH_EXECUTE\"")) || !bytes.Contains(body, []byte("MH_PIE = true")) {
		t.Fatalf("eza policy:\n%s", body)
	}
	if !bytes.Contains(body, []byte("MH_HAS_TLV_DESCRIPTORS")) || bytes.Contains(body, []byte("MH_WEAK_DEFINES")) || bytes.Contains(body, []byte("IOKit")) {
		t.Fatalf("eza flags:\n%s", body)
	}
	if bytes.Contains(body, []byte("/opt/homebrew")) || bytes.Contains(body, []byte("/usr/local/")) {
		t.Fatalf("eza names a Homebrew library:\n%s", body)
	}
	order := []string{
		"/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation",
		"/usr/lib/libSystem.B.dylib",
		"/usr/lib/libobjc.A.dylib",
	}
	at := 0
	for _, name := range order {
		needle := []byte("name = \"" + name + "\"")
		i := bytes.Index(body[at:], needle)
		if i < 0 {
			t.Fatalf("eza needed %s:\n%s", name, body)
		}
		at += i + len(needle)
	}
	if bytes.Count(body, []byte("why = ")) < len(order) {
		t.Fatalf("eza why:\n%s", body)
	}
}

func TestDarwinGlowPolicyRecordsTheNamedFile(t *testing.T) {
	body := readPolicy(t, "bininfo/policy/bin/glow.toml")
	if !bytes.Contains(body, []byte("format = \"macho\"")) || !bytes.Contains(body, []byte("filetype = \"MH_EXECUTE\"")) || !bytes.Contains(body, []byte("MH_PIE = true")) {
		t.Fatalf("glow policy:\n%s", body)
	}
	if bytes.Contains(body, []byte("MH_TWOLEVEL")) || bytes.Contains(body, []byte("IOKit")) || bytes.Contains(body, []byte("/opt/homebrew")) || bytes.Contains(body, []byte("/usr/local/")) {
		t.Fatalf("glow policy:\n%s", body)
	}
	order := []string{
		"/usr/lib/libSystem.B.dylib",
		"/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation",
		"/System/Library/Frameworks/Security.framework/Versions/A/Security",
	}
	at := 0
	for _, name := range order {
		needle := []byte("name = \"" + name + "\"")
		i := bytes.Index(body[at:], needle)
		if i < 0 {
			t.Fatalf("glow needed %s:\n%s", name, body)
		}
		at += i + len(needle)
	}
	if bytes.Count(body, []byte("why = ")) < len(order) {
		t.Fatalf("glow why:\n%s", body)
	}
}

func TestDarwinMakePolicyRecordsTheNamedFile(t *testing.T) {
	body := readPolicy(t, "bininfo/policy/bin/make.toml")
	if !bytes.Contains(body, []byte("format = \"macho\"")) || !bytes.Contains(body, []byte("filetype = \"MH_EXECUTE\"")) || !bytes.Contains(body, []byte("MH_PIE = true")) {
		t.Fatalf("make policy:\n%s", body)
	}
	if !bytes.Contains(body, []byte("MH_NOUNDEFS")) || !bytes.Contains(body, []byte("MH_TWOLEVEL")) {
		t.Fatalf("make flags:\n%s", body)
	}
	if bytes.Contains(body, []byte("IOKit")) || bytes.Contains(body, []byte("CoreFoundation")) || bytes.Contains(body, []byte("Security")) || bytes.Contains(body, []byte("/opt/homebrew")) || bytes.Contains(body, []byte("/usr/local/")) {
		t.Fatalf("make names an extra library:\n%s", body)
	}
	if !bytes.Contains(body, []byte("name = \"/usr/lib/libSystem.B.dylib\"")) || !bytes.Contains(body, []byte("why = ")) {
		t.Fatalf("make dylib:\n%s", body)
	}
}

func TestPolicyOverlayReplacesTheBaseFile(t *testing.T) {
	base := t.TempDir()
	overlay := t.TempDir()
	for _, dir := range []string{base, overlay} {
		if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	wrong := "format = \"elf\"\nDT_NEEDED = []\nDF_1_PIE = false\n"
	right := "format = \"elf\"\nDT_NEEDED = [{ name = \"libiconv.so.2\", why = \"The test records this library.\" }]\nDF_1_PIE = false\n"
	if err := os.WriteFile(filepath.Join(base, "bin", "tool.toml"), []byte(wrong), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(overlay, "bin", "tool.toml"), []byte(right), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "bin", "tool")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, IconvELF(), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BININFO_POLICY", base)
	t.Setenv("BININFO_POLICY_OVERLAY", overlay)
	if err := Tree(dir); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxBundlePoliciesRecordTheNamedFiles(t *testing.T) {
	static := []string{
		"bin/age",
		"bin/age-keygen",
		"bin/btop",
		"bin/direnv",
		"bin/fzf",
		"bin/glow",
		"bin/htop",
		"bin/jq",
		"bin/lazygit",
		"bin/make",
		"bin/shh",
		"nvim/bin/nvim",
		"zig/zig",
		"zsh/bin/zsh",
		"nvim/lib/nvim/parser/c.so",
		"nvim/lib/nvim/parser/query.so",
		"nvim/lib/nvim/parser/vimdoc.so",
	}
	for _, rel := range static {
		body := readPolicy(t, "bininfo/policy/linux/"+rel+".toml")
		if !staticElfPolicy(body, false) {
			t.Fatalf("%s is not a static ELF policy:\n%s", rel, body)
		}
	}
	for _, rel := range []string{
		"nvim/lib/nvim/parser/lua.so",
		"nvim/lib/nvim/parser/markdown.so",
		"nvim/lib/nvim/parser/markdown_inline.so",
		"nvim/lib/nvim/parser/vim.so",
	} {
		body := readPolicy(t, "bininfo/policy/linux/"+rel+".toml")
		if !bytes.Contains(body, []byte("format = \"elf\"")) || !bytes.Contains(body, []byte("name = \"libc.so\"")) || !bytes.Contains(body, []byte("why = ")) || !bytes.Contains(body, []byte("DF_1_PIE = false")) {
			t.Fatalf("%s parser policy:\n%s", rel, body)
		}
		if bytes.Contains(body, []byte("PT_INTERP = \"/")) || bytes.Contains(body, []byte("DF_1_PIE = true")) {
			t.Fatalf("%s names a loader:\n%s", rel, body)
		}
	}
	for _, rel := range []string{"bin/bat", "bin/dust", "bin/fd", "bin/rg", "bin/delta"} {
		if _, err := policyPath("bininfo/policy/linux/" + rel + ".toml"); err == nil {
			t.Fatalf("%s is in the shared policy", rel)
		}
	}
	for _, rel := range []string{"bin/bat", "bin/dust", "bin/fd", "bin/rg"} {
		arm := readPolicy(t, "bininfo/policy/linux-aarch64/"+rel+".toml")
		if !staticElfPolicy(arm, false) {
			t.Fatalf("aarch64 %s:\n%s", rel, arm)
		}
		x86 := readPolicy(t, "bininfo/policy/linux-x86_64/"+rel+".toml")
		if !staticElfPolicy(x86, true) {
			t.Fatalf("x86_64 %s:\n%s", rel, x86)
		}
	}
	armDelta := readPolicy(t, "bininfo/policy/linux-aarch64/bin/delta.toml")
	if !bytes.Contains(armDelta, []byte("PT_INTERP = \"/lib/ld-linux-aarch64.so.1\"")) || !bytes.Contains(armDelta, []byte("DF_1_PIE = true")) {
		t.Fatalf("aarch64 delta:\n%s", armDelta)
	}
	order := []string{
		"libgcc_s.so.1",
		"libpthread.so.0",
		"libm.so.6",
		"libdl.so.2",
		"libc.so.6",
		"ld-linux-aarch64.so.1",
	}
	at := 0
	for _, name := range order {
		needle := []byte("name = \"" + name + "\"")
		i := bytes.Index(armDelta[at:], needle)
		if i < 0 {
			t.Fatalf("aarch64 delta needed %s:\n%s", name, armDelta)
		}
		at += i + len(needle)
	}
	if bytes.Count(armDelta, []byte("why = ")) < len(order) {
		t.Fatalf("aarch64 delta why:\n%s", armDelta)
	}
	x86Delta := readPolicy(t, "bininfo/policy/linux-x86_64/bin/delta.toml")
	if !staticElfPolicy(x86Delta, true) {
		t.Fatalf("x86_64 delta:\n%s", x86Delta)
	}
}

func readPolicy(t *testing.T, rel string) []byte {
	t.Helper()
	path, err := policyPath(rel)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func staticElfPolicy(body []byte, pie bool) bool {
	if !bytes.Contains(body, []byte("format = \"elf\"")) || !bytes.Contains(body, []byte("DT_NEEDED = []")) {
		return false
	}
	if bytes.Contains(body, []byte("libc.so")) || bytes.Contains(body, []byte("PT_INTERP = \"/")) {
		return false
	}
	want := []byte("DF_1_PIE = false")
	other := []byte("DF_1_PIE = true")
	if pie {
		want = []byte("DF_1_PIE = true")
		other = []byte("DF_1_PIE = false")
	}
	return bytes.Contains(body, want) && !bytes.Contains(body, other)
}

func TestLinuxGitPoliciesRecordTheNamedFiles(t *testing.T) {
	for _, rel := range []string{
		"share/powerlevel10k/gitstatus/usrbin/gitstatusd-linux-aarch64",
		"share/powerlevel10k/gitstatus/usrbin/gitstatusd-linux-x86_64",
		"git/bin/git",
		"git/libexec/git-core/git",
		"git/libexec/git-core/git-add",
		"git/libexec/git-core/git-am",
		"git/libexec/git-core/git-annotate",
		"git/libexec/git-core/git-apply",
		"git/libexec/git-core/git-archive",
		"git/libexec/git-core/git-backfill",
		"git/libexec/git-core/git-bisect",
		"git/libexec/git-core/git-blame",
		"git/libexec/git-core/git-branch",
		"git/libexec/git-core/git-bugreport",
		"git/libexec/git-core/git-bundle",
		"git/libexec/git-core/git-cat-file",
		"git/libexec/git-core/git-check-attr",
		"git/libexec/git-core/git-check-ignore",
		"git/libexec/git-core/git-check-mailmap",
		"git/libexec/git-core/git-check-ref-format",
		"git/libexec/git-core/git-checkout",
		"git/libexec/git-core/git-checkout--worker",
		"git/libexec/git-core/git-checkout-index",
		"git/libexec/git-core/git-cherry",
		"git/libexec/git-core/git-cherry-pick",
		"git/libexec/git-core/git-clean",
		"git/libexec/git-core/git-clone",
		"git/libexec/git-core/git-column",
		"git/libexec/git-core/git-commit",
		"git/libexec/git-core/git-commit-graph",
		"git/libexec/git-core/git-commit-tree",
		"git/libexec/git-core/git-config",
		"git/libexec/git-core/git-count-objects",
		"git/libexec/git-core/git-credential",
		"git/libexec/git-core/git-credential-cache",
		"git/libexec/git-core/git-credential-cache--daemon",
		"git/libexec/git-core/git-credential-store",
		"git/libexec/git-core/git-daemon",
		"git/libexec/git-core/git-describe",
		"git/libexec/git-core/git-diagnose",
		"git/libexec/git-core/git-diff",
		"git/libexec/git-core/git-diff-files",
		"git/libexec/git-core/git-diff-index",
		"git/libexec/git-core/git-diff-pairs",
		"git/libexec/git-core/git-diff-tree",
		"git/libexec/git-core/git-difftool",
		"git/libexec/git-core/git-fast-export",
		"git/libexec/git-core/git-fast-import",
		"git/libexec/git-core/git-fetch",
		"git/libexec/git-core/git-fetch-pack",
		"git/libexec/git-core/git-fmt-merge-msg",
		"git/libexec/git-core/git-for-each-ref",
		"git/libexec/git-core/git-for-each-repo",
		"git/libexec/git-core/git-format-patch",
		"git/libexec/git-core/git-format-rev",
		"git/libexec/git-core/git-fsck",
		"git/libexec/git-core/git-fsck-objects",
		"git/libexec/git-core/git-fsmonitor--daemon",
		"git/libexec/git-core/git-gc",
		"git/libexec/git-core/git-get-tar-commit-id",
		"git/libexec/git-core/git-grep",
		"git/libexec/git-core/git-hash-object",
		"git/libexec/git-core/git-help",
		"git/libexec/git-core/git-history",
		"git/libexec/git-core/git-hook",
		"git/libexec/git-core/git-http-backend",
		"git/libexec/git-core/git-imap-send",
		"git/libexec/git-core/git-index-pack",
		"git/libexec/git-core/git-init",
		"git/libexec/git-core/git-init-db",
		"git/libexec/git-core/git-interpret-trailers",
		"git/libexec/git-core/git-last-modified",
		"git/libexec/git-core/git-log",
		"git/libexec/git-core/git-ls-files",
		"git/libexec/git-core/git-ls-remote",
		"git/libexec/git-core/git-ls-tree",
		"git/libexec/git-core/git-mailinfo",
		"git/libexec/git-core/git-mailsplit",
		"git/libexec/git-core/git-maintenance",
		"git/libexec/git-core/git-merge",
		"git/libexec/git-core/git-merge-base",
		"git/libexec/git-core/git-merge-file",
		"git/libexec/git-core/git-merge-index",
		"git/libexec/git-core/git-merge-ours",
		"git/libexec/git-core/git-merge-recursive",
		"git/libexec/git-core/git-merge-subtree",
		"git/libexec/git-core/git-merge-tree",
		"git/libexec/git-core/git-mktag",
		"git/libexec/git-core/git-mktree",
		"git/libexec/git-core/git-multi-pack-index",
		"git/libexec/git-core/git-mv",
		"git/libexec/git-core/git-name-rev",
		"git/libexec/git-core/git-notes",
		"git/libexec/git-core/git-pack-objects",
		"git/libexec/git-core/git-pack-redundant",
		"git/libexec/git-core/git-pack-refs",
		"git/libexec/git-core/git-patch-id",
		"git/libexec/git-core/git-prune",
		"git/libexec/git-core/git-prune-packed",
		"git/libexec/git-core/git-pull",
		"git/libexec/git-core/git-push",
		"git/libexec/git-core/git-range-diff",
		"git/libexec/git-core/git-read-tree",
		"git/libexec/git-core/git-rebase",
		"git/libexec/git-core/git-receive-pack",
		"git/libexec/git-core/git-reflog",
		"git/libexec/git-core/git-refs",
		"git/libexec/git-core/git-remote",
		"git/libexec/git-core/git-remote-ext",
		"git/libexec/git-core/git-remote-fd",
		"git/libexec/git-core/git-repack",
		"git/libexec/git-core/git-replace",
		"git/libexec/git-core/git-replay",
		"git/libexec/git-core/git-repo",
		"git/libexec/git-core/git-rerere",
		"git/libexec/git-core/git-reset",
		"git/libexec/git-core/git-restore",
		"git/libexec/git-core/git-rev-list",
		"git/libexec/git-core/git-rev-parse",
		"git/libexec/git-core/git-revert",
		"git/libexec/git-core/git-rm",
		"git/libexec/git-core/git-send-pack",
		"git/libexec/git-core/git-sh-i18n--envsubst",
		"git/libexec/git-core/git-shell",
		"git/libexec/git-core/git-shortlog",
		"git/libexec/git-core/git-show",
		"git/libexec/git-core/git-show-branch",
		"git/libexec/git-core/git-show-index",
		"git/libexec/git-core/git-show-ref",
		"git/libexec/git-core/git-sparse-checkout",
		"git/libexec/git-core/git-stage",
		"git/libexec/git-core/git-stash",
		"git/libexec/git-core/git-status",
		"git/libexec/git-core/git-stripspace",
		"git/libexec/git-core/git-submodule--helper",
		"git/libexec/git-core/git-switch",
		"git/libexec/git-core/git-symbolic-ref",
		"git/libexec/git-core/git-tag",
		"git/libexec/git-core/git-unpack-file",
		"git/libexec/git-core/git-unpack-objects",
		"git/libexec/git-core/git-update-index",
		"git/libexec/git-core/git-update-ref",
		"git/libexec/git-core/git-update-server-info",
		"git/libexec/git-core/git-upload-archive",
		"git/libexec/git-core/git-upload-pack",
		"git/libexec/git-core/git-url-parse",
		"git/libexec/git-core/git-var",
		"git/libexec/git-core/git-verify-commit",
		"git/libexec/git-core/git-verify-pack",
		"git/libexec/git-core/git-verify-tag",
		"git/libexec/git-core/git-version",
		"git/libexec/git-core/git-whatchanged",
		"git/libexec/git-core/git-worktree",
		"git/libexec/git-core/git-write-tree",
		"git/libexec/git-core/scalar",
	} {
		path, err := policyPath("bininfo/policy/linux/" + rel + ".toml")
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte("format = \"elf\"")) || !bytes.Contains(body, []byte("DT_NEEDED = []")) {
			t.Fatalf("%s is not a static ELF policy:\n%s", rel, body)
		}
		if bytes.Contains(body, []byte("libc.so")) || bytes.Contains(body, []byte("PT_INTERP = \"/")) {
			t.Fatalf("%s names a host library:\n%s", rel, body)
		}
		pie := []byte("DF_1_PIE = false")
		if bytes.Contains([]byte(rel), []byte("gitstatusd-linux-")) {
			pie = []byte("DF_1_PIE = true")
		}
		if !bytes.Contains(body, pie) {
			t.Fatalf("%s pie bit:\n%s", rel, body)
		}
	}
}

func TestTreeChecksABinaryInsideNvim(t *testing.T) {
	dir := t.TempDir()
	evil := filepath.Join(dir, "nvim", "bin", "not-shipped.exe")
	if err := os.MkdirAll(filepath.Dir(evil), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evil, IconvELF(), 0o755); err != nil {
		t.Fatal(err)
	}
	err := Tree(dir)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("nvim/bin/not-shipped.exe")) || !bytes.Contains([]byte(err.Error()), []byte("no policy")) {
		t.Fatalf("got %v", err)
	}
}

func TestTreeChecksABinaryInsideMinGit(t *testing.T) {
	dir := t.TempDir()
	evil := filepath.Join(dir, "git", "usr", "bin", "not-shipped.exe")
	if err := os.MkdirAll(filepath.Dir(evil), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evil, IconvELF(), 0o755); err != nil {
		t.Fatal(err)
	}
	err := Tree(dir)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("git/usr/bin/not-shipped.exe")) || !bytes.Contains([]byte(err.Error()), []byte("no policy")) {
		t.Fatalf("got %v", err)
	}
}

func TestTreeChecksABinaryNextToTheGoSDK(t *testing.T) {
	dir := t.TempDir()
	evil := filepath.Join(dir, "bin", "evil")
	if err := os.MkdirAll(filepath.Dir(evil), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evil, IconvELF(), 0o755); err != nil {
		t.Fatal(err)
	}
	err := Tree(dir)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("bin/evil")) || !bytes.Contains([]byte(err.Error()), []byte("no policy")) {
		t.Fatalf("got %v", err)
	}
}

func TestTreeSkipsScripts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cc"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("cc", filepath.Join(dir, "c++")); err != nil {
		t.Fatal(err)
	}
	if err := Tree(dir); err != nil {
		t.Fatal(err)
	}
}

func TestTreeRejectsOutsideSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "host")
	if err := os.WriteFile(outside, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "htop")); err != nil {
		t.Fatal(err)
	}
	err := Tree(dir)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("leaves the tree")) {
		t.Fatalf("got %v", err)
	}
}

func TestArchiveRejectsOutsideSymlink(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	hdr := &tar.Header{Name: "bin/htop", Typeflag: tar.TypeSymlink, Linkname: "/usr/bin/htop", Mode: 0o777}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.tar.gz")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Archive(path)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("bin/htop")) || !bytes.Contains([]byte(err.Error()), []byte("leaves the tree")) {
		t.Fatalf("got %v", err)
	}
}

func TestZipNamesTheMember(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	body := IconvELF()
	hdr := &zip.FileHeader{Name: "bin/not-shipped.exe"}
	hdr.SetMode(0o755)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	err = Archive(path)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("bin/not-shipped.exe")) || !bytes.Contains([]byte(err.Error()), []byte("no policy")) {
		t.Fatalf("got %v", err)
	}
}
