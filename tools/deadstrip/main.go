// Command deadstrip reads one file and writes it back with unused Mach-O dylib loads removed.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/stefanpenner/devlayer/internal/deadstrip"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("deadstrip", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	in := fs.String("in", "", "input file")
	out := fs.String("out", "", "output file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" || *out == "" {
		return errors.New("deadstrip: need --in and --out")
	}
	body, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	stripped, changed, err := deadstrip.Strip(body)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, stripped, 0o755); err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return deadstrip.Sign(*out)
}
