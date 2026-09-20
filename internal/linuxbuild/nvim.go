package linuxbuild

import "io"

// Nvim clones the pinned tag v${NVIM_VERSION}, installs to /opt/nvim, tars.
func Nvim(x Exec, out Output, env map[string]string, stdout io.Writer) error {
	ver, err := requireEnv(env, "NVIM_VERSION")
	if err != nil {
		return err
	}
	if err := x("", "git", "clone", "--depth", "1", "--branch", "v"+ver, "https://github.com/neovim/neovim.git"); err != nil {
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
