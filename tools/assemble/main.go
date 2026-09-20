package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/stefanpenner/devlayer/internal/assemble"
	"github.com/stefanpenner/devlayer/internal/binaries"
	"github.com/stefanpenner/devlayer/internal/versions"
)

func main() {
	if d := os.Getenv("BUILD_WORKING_DIRECTORY"); d != "" {
		_ = os.Chdir(d)
	}

	out := flag.String("out", "", "output tar.gz")
	arch := flag.String("arch", "", "linux arch (x86_64 or aarch64)")
	versionsPath := flag.String("versions", "", "versions.env")
	git := flag.String("git", "", "git.tar.gz")
	zsh := flag.String("zsh", "", "zsh.tar.gz")
	htop := flag.String("htop", "", "htop.tar.gz")
	btop := flag.String("btop", "", "btop.tar.gz")
	nvim := flag.String("nvim", "", "nvim.tar.gz")
	makeTar := flag.String("make", "", "make.tar.gz")
	flag.Parse()

	if *out == "" || *arch == "" || *versionsPath == "" ||
		*git == "" || *zsh == "" || *htop == "" || *btop == "" || *nvim == "" || *makeTar == "" {
		fmt.Fprintln(os.Stderr, "usage: assemble --out OUT.tar.gz --arch ARCH --versions versions.env --git git.tar.gz --zsh zsh.tar.gz --htop htop.tar.gz --btop btop.tar.gz --nvim nvim.tar.gz --make make.tar.gz")
		os.Exit(2)
	}

	data, err := os.ReadFile(*versionsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = assemble.Run(*out, *arch, versions.Parse(string(data)), assemble.Compiled{
		Git:  *git,
		Zsh:  *zsh,
		Htop: *htop,
		Btop: *btop,
		Nvim: *nvim,
		Make: *makeTar,
	}, binaries.Fetch)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
