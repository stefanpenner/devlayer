package linuxbuild

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

const GitConfigMak = `prefix = /opt/git
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

// Git fetches, configures, makes, strips, and tars git to stdout.
func Git(x Exec, out Output, env map[string]string, stdout io.Writer) error {
	ver, err := requireEnv(env, "GIT_VERSION")
	if err != nil {
		return err
	}
	dir := "git-" + ver

	if err := fetchTarball(x, "https://github.com/git/git/archive/refs/tags/v"+ver+".tar.gz", dir+".tar.gz"); err != nil {
		return err
	}

	if err := writeGitConfigMak(dir, out); err != nil {
		return err
	}

	if err := x(dir, "make", jobs()); err != nil {
		return err
	}
	if err := x(dir, "make", "install"); err != nil {
		return err
	}

	if err := stripGit(x); err != nil {
		return err
	}

	return tarPrefix(x, "/opt/git")
}

func writeGitConfigMak(dir string, out Output) error {
	libs, err := out("", "pkg-config", "--static", "--libs", "libcurl")
	if err != nil {
		return err
	}
	body := GitConfigMak + "CURL_LDFLAGS = " + StripLdl(strings.TrimSpace(libs)) + "\n"
	return os.WriteFile(filepath.Join(dir, "config.mak"), []byte(body), 0644)
}

func stripGit(x Exec) error {
	if err := x("", "strip", "/opt/git/bin/git"); err != nil {
		return err
	}
	return StripELF(x, "/opt/git/libexec", "git")
}

// StripLdl drops -ldl tokens from a pkg-config libs line.
func StripLdl(s string) string {
	fields := strings.Fields(s)
	keep := make([]string, 0, len(fields))
	for _, f := range fields {
		if f == "-ldl" {
			continue
		}
		keep = append(keep, f)
	}
	return strings.Join(keep, " ")
}

// StripELF runs strip on regular ELF files under root whose names start with prefix.
func StripELF(x Exec, root, prefix string) error {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if !strings.HasPrefix(info.Name(), prefix) {
			return nil
		}
		if !isELF(path) {
			return nil
		}
		return x("", "strip", path)
	})
}

func isELF(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return false
	}
	return magic[0] == 0x7f && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F'
}
