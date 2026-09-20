package linuxbuild

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestEnableStaticModules(t *testing.T) {
	in := `name=zsh/complete     link=no     load=no     auto=yes
name=zsh/completer    link=no     load=no
name=zsh/zle          link=no     load=no
name=zsh/net/socket   link=no     load=no
name=zsh/net/tcp      link=no     load=no
name=zsh/compctl      link=dynamic load=yes
name=zsh/other        link=no     load=no
`
	got := EnableStaticModules(in, []string{
		"complete", "zle", "net/socket", "net/tcp", "compctl",
	})

	wantLines := map[string]string{
		"name=zsh/complete":   "link=static",
		"name=zsh/completer":  "link=no",
		"name=zsh/zle":        "link=static",
		"name=zsh/net/socket": "link=static",
		"name=zsh/net/tcp":    "link=static",
		"name=zsh/compctl":    "link=dynamic",
		"name=zsh/other":      "link=no",
	}
	wantLoad := map[string]string{
		"name=zsh/complete":   "load=yes",
		"name=zsh/completer":  "load=no",
		"name=zsh/zle":        "load=yes",
		"name=zsh/net/socket": "load=yes",
		"name=zsh/net/tcp":    "load=yes",
		"name=zsh/other":      "load=no",
	}

	for _, line := range strings.Split(got, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var key string
		for k := range wantLines {
			if strings.Contains(line, k) && (key == "" || len(k) > len(key)) {
				key = k
			}
		}
		if key == "" {
			t.Fatalf("unexpected line: %q", line)
		}
		if !strings.Contains(line, wantLines[key]) {
			t.Errorf("%s: got %q, want %s", key, line, wantLines[key])
		}
		if load, ok := wantLoad[key]; ok && !strings.Contains(line, load) {
			t.Errorf("%s: got %q, want %s", key, line, load)
		}
	}
}

func TestZsh(t *testing.T) {
	chdirTemp(t)
	if err := os.MkdirAll("zsh", 0755); err != nil {
		t.Fatal(err)
	}
	mod := `name=zsh/complete link=no load=no
name=zsh/net/socket link=no load=no
name=zsh/other link=no load=no
`
	if err := os.WriteFile("zsh/config.modules", []byte(mod), 0644); err != nil {
		t.Fatal(err)
	}

	var r recorder
	if err := Zsh(r.exec, r.output, nil, io.Discard); err != nil {
		t.Fatal(err)
	}

	r.hasSeq(t,
		"git clone --depth 1 https://github.com/zsh-users/zsh.git",
		"zsh: ./Util/preconfig",
		"zsh: ./configure --prefix=/opt/zsh --enable-static --disable-dynamic --enable-multibyte --with-tcsetpgrp LDFLAGS=-static CFLAGS=-Os -DNDEBUG",
		"zsh: make "+jobsArg(),
		"zsh: make install.bin install.fns",
		"strip /opt/zsh/bin/zsh",
		"tar czf - -C /opt/zsh .",
	)
	for _, c := range r.cmds {
		if strings.Contains(c, "git clone") && strings.Contains(c, "--branch") {
			t.Fatalf("zsh clone should be master, not a branch: %s", c)
		}
	}

	body, err := os.ReadFile("zsh/config.modules")
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, "name=zsh/complete") || !strings.Contains(got, "link=static") {
		t.Fatalf("complete not static:\n%s", got)
	}
	if !strings.Contains(got, "name=zsh/net/socket") || !strings.Contains(got, "link=static") {
		t.Fatalf("net/socket not static:\n%s", got)
	}
	if !strings.Contains(got, "name=zsh/other link=no load=no") {
		t.Fatalf("other should be unchanged:\n%s", got)
	}
}
