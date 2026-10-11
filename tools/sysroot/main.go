package main

import (
	"fmt"
	"os"

	"github.com/stefanpenner/devlayer/internal/sysroot"
)

func main() {
	arch := "x86_64"
	out := "output/sysroot"
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-arch":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "usage: sysroot [-arch x86_64|aarch64] [-o dir]")
				os.Exit(2)
			}
			arch = args[i]
		case "-o":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "usage: sysroot [-arch x86_64|aarch64] [-o dir]")
				os.Exit(2)
			}
			out = args[i]
		default:
			fmt.Fprintf(os.Stderr, "unknown argument %s\n", args[i])
			os.Exit(2)
		}
	}
	if _, err := sysroot.Build(arch, out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
