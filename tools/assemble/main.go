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
	var bins, trees, files stringList
	flag.Var(&bins, "bin", "copy to bin, name=path")
	flag.Var(&trees, "tree", "copy a directory, dest=path")
	flag.Var(&files, "file", "copy one file, dest=path")
	flag.Parse()

	if *out == "" || *arch == "" ||
		*git == "" || *zsh == "" || *htop == "" || *btop == "" || *nvim == "" || *makeTar == "" {
		fmt.Fprintln(os.Stderr, "usage: assemble --out OUT.tar.gz --arch ARCH --git git.tar.gz --zsh zsh.tar.gz --htop htop.tar.gz --btop btop.tar.gz --nvim nvim.tar.gz --make make.tar.gz [--bin name=path] [--tree dest=path] [--file dest=path]")
		os.Exit(2)
	}

	compiled := assemble.Compiled{
		Git:  *git,
		Zsh:  *zsh,
		Htop: *htop,
		Btop: *btop,
		Nvim: *nvim,
		Make: *makeTar,
	}
	var err error
	if len(bins)+len(trees)+len(files) > 0 {
		in, perr := placedInput(bins, trees, files)
		if perr != nil {
			fmt.Fprintln(os.Stderr, perr)
			os.Exit(2)
		}
		err = assemble.RunPlaced(*out, *arch, compiled, in)
	} else {
		if *versionsPath == "" {
			fmt.Fprintln(os.Stderr, "assemble: --versions is required when no --bin, --tree, or --file is set")
			os.Exit(2)
		}
		data, rerr := os.ReadFile(*versionsPath)
		if rerr != nil {
			fmt.Fprintln(os.Stderr, rerr)
			os.Exit(1)
		}
		err = assemble.Run(*out, *arch, versions.Parse(string(data)), compiled, binaries.Fetch)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type stringList []string

func (s *stringList) String() string { return "" }

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func placedInput(bins, trees, files []string) (assemble.Input, error) {
	var in assemble.Input
	for _, spec := range bins {
		name, path, err := assemble.ParseNamed(spec)
		if err != nil {
			return in, err
		}
		in.Bins = append(in.Bins, assemble.Bin{Name: name, Path: path})
	}
	for _, spec := range trees {
		dest, path, err := assemble.ParseNamed(spec)
		if err != nil {
			return in, err
		}
		in.Trees = append(in.Trees, assemble.Tree{Dest: dest, Path: path})
	}
	for _, spec := range files {
		dest, path, err := assemble.ParseNamed(spec)
		if err != nil {
			return in, err
		}
		in.Files = append(in.Files, assemble.File{Dest: dest, Path: path})
	}
	return in, nil
}
