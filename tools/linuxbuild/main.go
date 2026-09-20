package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/stefanpenner/devlayer/internal/linuxbuild"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: linuxbuild <tool>")
		os.Exit(1)
	}
	if err := linuxbuild.Run(os.Args[1], environ(os.Environ()), os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func environ(kv []string) map[string]string {
	m := make(map[string]string, len(kv))
	for _, e := range kv {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			m[k] = v
		}
	}
	return m
}
