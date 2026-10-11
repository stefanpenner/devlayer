package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/stefanpenner/devlayer/internal/archive"
	"github.com/stefanpenner/devlayer/internal/config"
	"github.com/stefanpenner/devlayer/internal/ssh"
)

const privateTarName = "devlayer-private.tar.gz"

// buildPrivate clones or updates the private checkout and packs it.
// No private.toml means there is no private layer.
func buildPrivate(outDir string) error {
	cfg, err := config.LoadPrivate()
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil
	}

	fmt.Println("==> Packaging private layer...")
	if err := ensureCheckout(cfg.Repo, cfg.Path); err != nil {
		return err
	}
	tarPath := filepath.Join(outDir, privateTarName)
	if err := packPrivate(cfg.Path, tarPath); err != nil {
		return err
	}
	printSize(tarPath)
	return nil
}

func ensureCheckout(repo, dest string) error {
	if isGitCheckout(dest) {
		return runGit(dest, "pull", "--ff-only")
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("private path %s exists and is not a git checkout", dest)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	return runGit("", "clone", repo, dest)
}

func isGitCheckout(dest string) bool {
	cmd := exec.Command("git", "-C", dest, "rev-parse", "--is-inside-work-tree")
	cmd.Env = gitEnv()
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = gitEnv()
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Print(string(out))
	}
	if err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func gitEnv() []string {
	return append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
}

// packPrivate writes dest's tree into tarPath under .ai-private/.
// .git stays out of the tarball.
func packPrivate(src, tarPath string) error {
	staging, err := os.MkdirTemp("", "devlayer-private-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	dst := filepath.Join(staging, ".ai-private")
	if err := copyDirSimple(src, dst); err != nil {
		return fmt.Errorf("copy private layer: %w", err)
	}
	return archive.CreateTarGz(tarPath, staging)
}

// installPrivate extracts the private tarball into home and runs install.sh.
func installPrivate(scriptDir, home string) error {
	tarPath := filepath.Join(scriptDir, privateTarName)
	if _, err := os.Stat(tarPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	fmt.Println("==> Installing private layer...")
	if err := archive.ExtractTarGz(tarPath, home); err != nil {
		return fmt.Errorf("private layer: %w", err)
	}
	return runPrivateInstall(filepath.Join(home, ".ai-private", "install.sh"))
}

func runPrivateInstall(script string) error {
	info, err := os.Stat(script)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", script)
	}
	fmt.Printf("==> Running %s\n", script)
	cmd := exec.Command(script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("private install: %w", err)
	}
	return nil
}

func deployPrivate(host, scriptDir string) error {
	tarPath := filepath.Join(scriptDir, privateTarName)
	if _, err := os.Stat(tarPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	fmt.Println("==> Deploying private layer...")
	if err := ssh.PipeFile(host, tarPath, "tar xzf - -C $HOME"); err != nil {
		return fmt.Errorf("private layer: %w", err)
	}
	err := ssh.RunInteractive(host, `if [ -f "$HOME/.ai-private/install.sh" ]; then "$HOME/.ai-private/install.sh"; fi`)
	if err != nil {
		return fmt.Errorf("private install: %w", err)
	}
	return nil
}
