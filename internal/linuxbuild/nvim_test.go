package linuxbuild

import (
	"io"
	"strings"
	"testing"
)

func TestNvim(t *testing.T) {
	chdirTemp(t)
	var r recorder
	env := map[string]string{"NVIM_VERSION": "0.0.1"}
	if err := Nvim(r.exec, r.output, env, io.Discard); err != nil {
		t.Fatal(err)
	}

	r.hasSeq(t,
		"git clone --depth 1 --branch stable https://github.com/neovim/neovim.git",
		"neovim: make CMAKE_BUILD_TYPE=Release CMAKE_EXTRA_FLAGS=-DCMAKE_INSTALL_PREFIX=/opt/nvim -DCMAKE_EXE_LINKER_FLAGS='-static -Wl,--export-dynamic' "+jobsArg(),
		"neovim: make install",
		"strip /opt/nvim/bin/nvim",
		"tar czf - -C /opt/nvim .",
	)
	for _, c := range r.cmds {
		if strings.Contains(c, "0.0.1") {
			t.Fatalf("nvim must use branch stable, not NVIM_VERSION: %s", c)
		}
	}
}
