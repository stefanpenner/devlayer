package linuxbuild

import (
	"io"
	"strings"
	"testing"
)

func TestNvimMissingVersion(t *testing.T) {
	var r recorder
	err := Nvim(r.exec, r.output, nil, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "NVIM_VERSION") {
		t.Fatalf("got %v, want missing NVIM_VERSION", err)
	}
}

func TestNvim(t *testing.T) {
	chdirTemp(t)
	var r recorder
	env := map[string]string{"NVIM_VERSION": "0.12.5"}
	if err := Nvim(r.exec, r.output, env, io.Discard); err != nil {
		t.Fatal(err)
	}

	r.hasSeq(t,
		"git clone --depth 1 --branch v0.12.5 https://github.com/neovim/neovim.git",
		"neovim: make CMAKE_BUILD_TYPE=Release CMAKE_EXTRA_FLAGS=-DCMAKE_INSTALL_PREFIX=/opt/nvim -DCMAKE_EXE_LINKER_FLAGS='-static -Wl,--export-dynamic' "+jobsArg(),
		"neovim: make install",
		"strip /opt/nvim/bin/nvim",
		"tar czf - -C /opt/nvim .",
	)
}
