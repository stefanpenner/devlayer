package cmd

import (
	"flag"
	"os"

	"github.com/stefanpenner/devlayer/internal/checkupdates"
)

func CheckUpdates(args []string) error {
	fs := flag.NewFlagSet("check-updates", flag.ContinueOnError)
	envPath := fs.String("env", "versions.env", "path to versions.env")
	dry := fs.Bool("dry-run", false, "report bumps without rewriting versions.env")
	if err := fs.Parse(args); err != nil {
		return err
	}

	_, err := checkupdates.Run(*envPath, checkupdates.NetGet, *dry, os.Getenv, os.Stdout)
	return err
}
