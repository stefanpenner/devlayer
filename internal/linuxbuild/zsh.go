package linuxbuild

import (
	"io"
	"os"
	"strings"
)

var zshStaticModules = []string{
	"compctl", "complete", "complist", "computil",
	"zle", "zutil", "parameter", "terminfo",
	"datetime", "stat", "system", "mathfunc",
	"net/socket", "net/tcp",
}

// Zsh clones the pinned tag zsh-$ZSH_VERSION, static-links selected modules, tars.
func Zsh(x Exec, out Output, env map[string]string, stdout io.Writer) error {
	ver, err := requireEnv(env, "ZSH_VERSION")
	if err != nil {
		return err
	}
	if err := x("", "git", "clone", "--depth", "1", "--branch", "zsh-"+ver, "https://github.com/zsh-users/zsh.git"); err != nil {
		return err
	}

	if err := x("zsh", "./Util/preconfig"); err != nil {
		return err
	}
	if err := x("zsh", "./configure",
		"--prefix=/opt/zsh",
		"--enable-static",
		"--disable-dynamic",
		"--enable-multibyte",
		"--with-tcsetpgrp",
		"LDFLAGS=-static",
		"CFLAGS=-Os -DNDEBUG",
	); err != nil {
		return err
	}

	if err := rewriteZshModules("zsh/config.modules"); err != nil {
		return err
	}

	if err := x("zsh", "make", jobs()); err != nil {
		return err
	}
	if err := x("zsh", "make", "install.bin", "install.fns"); err != nil {
		return err
	}

	if err := x("", "strip", "/opt/zsh/bin/zsh"); err != nil {
		return err
	}
	return tarPrefix(x, "/opt/zsh")
}

func rewriteZshModules(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(EnableStaticModules(string(b), zshStaticModules)), 0644)
}

// EnableStaticModules sets link=static load=yes on matching name=zsh/X lines.
func EnableStaticModules(text string, modules []string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		for _, mod := range modules {
			if !lineHasModule(line, mod) {
				continue
			}
			line = strings.ReplaceAll(line, "link=no", "link=static")
			line = strings.ReplaceAll(line, "load=no", "load=yes")
			break
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func lineHasModule(line, mod string) bool {
	needle := "name=zsh/" + mod
	for {
		i := strings.Index(line, needle)
		if i < 0 {
			return false
		}
		end := i + len(needle)
		if end == len(line) || line[end] == ' ' || line[end] == '\t' {
			return true
		}
		line = line[i+1:]
	}
}
