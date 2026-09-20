package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/stefanpenner/devlayer/internal/checkupdates"
)

func main() {
	chdirWorkspace()

	envPath := flag.String("env", "versions.env", "path to versions.env")
	dry := flag.Bool("dry-run", false, "report bumps without rewriting versions.env")
	flag.Parse()

	if _, err := checkupdates.Run(*envPath, checkupdates.NetGet, *dry, os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func chdirWorkspace() {
	if d := os.Getenv("BUILD_WORKING_DIRECTORY"); d != "" {
		_ = os.Chdir(d)
	}
}
