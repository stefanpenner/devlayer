package linuxbuild

import "io"

// Htop clones the pinned tag, builds a static binary, and tars the file to stdout.
func Htop(x Exec, out Output, env map[string]string, stdout io.Writer) error {
	ver, err := requireEnv(env, "HTOP_VERSION")
	if err != nil {
		return err
	}

	if err := x("", "git", "clone", "--depth", "1", "--branch", ver, "https://github.com/htop-dev/htop.git"); err != nil {
		return err
	}

	if err := x("htop", "./autogen.sh"); err != nil {
		return err
	}
	if err := x("htop", "./configure", "--enable-static", "LDFLAGS=-static", "CFLAGS=-Os -DNDEBUG"); err != nil {
		return err
	}

	if err := x("htop", "make", jobs()); err != nil {
		return err
	}
	if err := x("htop", "strip", "htop"); err != nil {
		return err
	}
	return x("htop", "tar", "czf", "-", "htop")
}
