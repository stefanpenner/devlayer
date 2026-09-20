package linuxbuild

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripLdl(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"-lcurl -lssl", "-lcurl -lssl"},
		{"-lcurl -ldl -lssl", "-lcurl -lssl"},
		{"-ldl", ""},
		{"-ldl -lcurl -ldl", "-lcurl"},
		{"-lcurl\t-ldl\t-lssl", "-lcurl -lssl"},
		{"  -ldl  -lcurl  ", "-lcurl"},
	}
	for _, tt := range tests {
		got := StripLdl(tt.in)
		if got != tt.want {
			t.Errorf("StripLdl(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestGitConfigMak(t *testing.T) {
	want := `prefix = /opt/git
NO_TCLTK = YesPlease
NO_GETTEXT = YesPlease
NO_PERL = YesPlease
NO_PYTHON = YesPlease
NO_EXPAT = YesPlease
NO_NSEC = YesPlease
NO_REGEX = YesPlease
NO_RUST = YesPlease
CFLAGS = -Os -DNDEBUG
LDFLAGS = -static -Wl,--allow-multiple-definition
`
	if GitConfigMak != want {
		t.Fatalf("GitConfigMak mismatch\n got: %q\nwant: %q", GitConfigMak, want)
	}
}

func TestGitMissingVersion(t *testing.T) {
	var r recorder
	err := Git(r.exec, r.output, nil, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "GIT_VERSION") {
		t.Fatalf("got %v, want missing GIT_VERSION", err)
	}
	if len(r.cmds) != 0 {
		t.Fatalf("ran commands on missing env: %v", r.cmds)
	}
}

func TestGit(t *testing.T) {
	chdirTemp(t)
	ver := "2.53.0"
	dir := "git-" + ver
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	r := recorder{out: "-lcurl -ldl -lssl -lcrypto"}
	if err := Git(r.exec, r.output, map[string]string{"GIT_VERSION": ver}, io.Discard); err != nil {
		t.Fatal(err)
	}

	url := "https://github.com/git/git/archive/refs/tags/v" + ver + ".tar.gz"
	r.has(t, url)
	r.hasSeq(t,
		"curl",
		"tar",
		"pkg-config --static --libs libcurl",
		dir+": make "+jobsArg(),
		dir+": make install",
		"strip /opt/git/bin/git",
		"tar czf - -C /opt/git .",
	)

	body, err := os.ReadFile(filepath.Join(dir, "config.mak"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.HasPrefix(got, GitConfigMak) {
		t.Fatalf("config.mak missing static body:\n%s", got)
	}
	if !strings.Contains(got, "CURL_LDFLAGS = -lcurl -lssl -lcrypto") {
		t.Fatalf("config.mak missing stripped CURL_LDFLAGS:\n%s", got)
	}
	if strings.Contains(got, "-ldl") {
		t.Fatalf("config.mak still has -ldl:\n%s", got)
	}
}

func TestStripELF(t *testing.T) {
	root := t.TempDir()
	elf := filepath.Join(root, "git-foo")
	script := filepath.Join(root, "git-script")
	other := filepath.Join(root, "notgit")
	nested := filepath.Join(root, "core")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	nestedELF := filepath.Join(nested, "git-remote-http")

	if err := os.WriteFile(elf, append([]byte{0x7f, 'E', 'L', 'F'}, []byte("bin")...), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, append([]byte{0x7f, 'E', 'L', 'F'}, []byte("x")...), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nestedELF, append([]byte{0x7f, 'E', 'L', 'F'}, []byte("x")...), 0755); err != nil {
		t.Fatal(err)
	}

	var r recorder
	if err := StripELF(r.exec, root, "git"); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(r.cmds, "\n")
	if !strings.Contains(got, "strip "+elf) {
		t.Fatalf("missing strip %s in %v", elf, r.cmds)
	}
	if !strings.Contains(got, "strip "+nestedELF) {
		t.Fatalf("missing strip %s in %v", nestedELF, r.cmds)
	}
	if strings.Contains(got, script) {
		t.Fatalf("stripped non-ELF: %v", r.cmds)
	}
	if strings.Contains(got, other) {
		t.Fatalf("stripped non-git* ELF: %v", r.cmds)
	}
}

func TestStripELFMissingDir(t *testing.T) {
	err := StripELF(func(dir, name string, args ...string) error {
		t.Fatal("should not exec")
		return nil
	}, filepath.Join(t.TempDir(), "missing"), "git")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGitStdoutUnusedByRecipe(t *testing.T) {
	// tar is recorded via Exec; recipe must not write the archive itself
	chdirTemp(t)
	if err := os.MkdirAll("git-1", 0755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	r := recorder{out: "-lcurl"}
	if err := Git(r.exec, r.output, map[string]string{"GIT_VERSION": "1"}, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("recipe wrote %d bytes to stdout; tar should be Exec", buf.Len())
	}
}
