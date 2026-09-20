package linuxbuild

import "io"

// Make fetches GNU make, builds static, and tars the binary to stdout.
func Make(x Exec, out Output, env map[string]string, stdout io.Writer) error {
	ver, err := requireEnv(env, "MAKE_VERSION")
	if err != nil {
		return err
	}
	dir := "make-" + ver

	if err := fetchTarball(x, "https://ftp.gnu.org/gnu/make/make-"+ver+".tar.gz", dir+".tar.gz"); err != nil {
		return err
	}

	if err := x(dir, "./configure", "CFLAGS=-Os -DNDEBUG", "LDFLAGS=-static"); err != nil {
		return err
	}
	if err := x(dir, "make", jobs()); err != nil {
		return err
	}
	if err := x(dir, "strip", "make"); err != nil {
		return err
	}
	return x(dir, "tar", "czf", "-", "make")
}
