package linuxbuild

import "io"

// Nvim clones branch stable (not the version pin), installs to /opt/nvim, tars.
func Nvim(x Exec, out Output, env map[string]string, stdout io.Writer) error {
	if err := x("", "git", "clone", "--depth", "1", "--branch", "stable", "https://github.com/neovim/neovim.git"); err != nil {
		return err
	}

	extra := "-DCMAKE_INSTALL_PREFIX=/opt/nvim -DCMAKE_EXE_LINKER_FLAGS='-static -Wl,--export-dynamic'"
	if err := x("neovim", "make",
		"CMAKE_BUILD_TYPE=Release",
		"CMAKE_EXTRA_FLAGS="+extra,
		jobs(),
	); err != nil {
		return err
	}
	if err := x("neovim", "make", "install"); err != nil {
		return err
	}

	if err := x("", "strip", "/opt/nvim/bin/nvim"); err != nil {
		return err
	}
	return tarPrefix(x, "/opt/nvim")
}
