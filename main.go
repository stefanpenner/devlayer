package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/stefanpenner/devlayer/cmd"
	"github.com/stefanpenner/devlayer/internal/versions"
)

//go:embed versions.env
var versionsEnv string

//go:embed Dockerfile
var dockerfile string

// Version is set at build time via -ldflags.
var Version = "dev"

func usage() {
	fmt.Print(`Usage: devlayer <command> [options]

Commands:
  build [--os OS] [--arch ARCH]   Build bundle + dotfiles + nvim plugins
  push <host>                     Deploy everything to remote host via SSH
  status [host]                   Show installed tool versions (local or SSH)
  install                         Install bundle locally (DEVLAYER_PREFIX, default ~/.local)
  init [--force]                  Write ~/.config/devlayer/config.toml
  doctor                          Check the local install
  ls                              List installed tools, dotfiles, and nvim plugins
  clean                           Remove build artifacts and Docker image
  upgrade                         Download and install the latest release
  check-updates [--dry-run]       Fetch latest tool versions; rewrite versions.env
  version                         Print devlayer version
  versions                        Print bundled tool versions

Options:
  OS: linux, darwin, or windows (default: this machine)
  ARCH defaults to current machine architecture
  DEVLAYER_PREFIX env var controls install location
    (default: ~/.local on unix, %LOCALAPPDATA%\devlayer on Windows)
  DEVLAYER_NO_AUTOUPDATE=1 disables automatic update checks

Examples:
  devlayer build                   Build for linux/current-arch
  devlayer build --os darwin       Build for darwin/current-arch
  devlayer build --os windows      Build for windows/current-arch
  devlayer push nas                Deploy to NAS
  devlayer status nas              Check versions on NAS
  devlayer install                 Install locally
`)
}

func main() {
	if d := os.Getenv("BUILD_WORKING_DIRECTORY"); d != "" {
		_ = os.Chdir(d)
	}

	if len(os.Args) < 2 {
		usage()
		return
	}

	vers := versions.Parse(versionsEnv)

	scriptDir, isTmp, err := cmd.FindScriptDir(dockerfile, versionsEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if isTmp {
		defer os.RemoveAll(scriptDir)
	}

	// Auto-update before running the command (skip for upgrade/version).
	switch os.Args[1] {
	case "upgrade", "version", "versions", "init", "doctor", "check-updates":
		// no auto-update
	default:
		cmd.AutoUpdate(Version)
	}

	switch os.Args[1] {
	case "build":
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			err = cwdErr
			break
		}
		err = cmd.Build(os.Args[2:], vers, scriptDir, cwd)
	case "init":
		force := false
		if len(os.Args) > 2 && os.Args[2] == "--force" {
			force = true
		}
		err = cmd.Init(force)
	case "doctor":
		err = cmd.Doctor()
	case "push":
		host := ""
		if len(os.Args) > 2 {
			host = os.Args[2]
		}
		err = cmd.Push(host, scriptDir, vers)
	case "status":
		host := ""
		if len(os.Args) > 2 {
			host = os.Args[2]
		}
		err = cmd.Status(host)
	case "install":
		err = cmd.Install(scriptDir)
	case "ls":
		err = cmd.Ls()
	case "clean":
		err = cmd.Clean(scriptDir)
	case "upgrade":
		err = cmd.Upgrade(Version)
	case "check-updates":
		err = cmd.CheckUpdates(os.Args[2:])
	case "version":
		fmt.Println(Version)
	case "versions":
		fmt.Print(cmd.VersionSummary(vers))
	default:
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
