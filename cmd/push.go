package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/stefanpenner/devlayer/internal/ssh"
	"github.com/stefanpenner/devlayer/internal/versions"
)

// Push deploys a devlayer bundle to a remote host via SSH.
func Push(host, scriptDir string, vers *versions.Versions) error {
	if host == "" {
		return fmt.Errorf("usage: devlayer push <host>")
	}

	// Get remote architecture
	arch, err := ssh.Run(host, "uname -m")
	if err != nil {
		return fmt.Errorf("failed to detect remote arch: %w", err)
	}

	bundleName := fmt.Sprintf("devlayer-linux-%s.tar.gz", arch)
	tarball, err := findBundle(bundleName, scriptDir)
	if err != nil {
		fmt.Printf("==> No bundle found, building for linux/%s...\n", arch)
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return cwdErr
		}
		if err := Build([]string{"--os", "linux", "--arch", arch}, vers, scriptDir, cwd); err != nil {
			return fmt.Errorf("auto-build failed: %w", err)
		}
		tarball, err = findBundle(bundleName, scriptDir)
		if err != nil {
			return err
		}
	}

	// Get remote prefix
	remotePrefix, err := ssh.Run(host, `echo "${DEVLAYER_PREFIX:-$HOME/.local}"`)
	if err != nil {
		remotePrefix = "$HOME/.local"
	}

	fmt.Printf("==> Deploying to %s (%s)...\n", host, remotePrefix)

	// Create directory and extract
	if err := ssh.RunInteractive(host, fmt.Sprintf("mkdir -p '%s/bin'", remotePrefix)); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	if err := ssh.PipeFile(host, tarball, fmt.Sprintf("tar xzf - -C '%s'", remotePrefix)); err != nil {
		return fmt.Errorf("tar extract: %w", err)
	}

	// Deploy dotfiles (if built)
	dotfilesTar := filepath.Join(scriptDir, "devlayer-dotfiles.tar.gz")
	if _, err := os.Stat(dotfilesTar); err == nil {
		fmt.Println("==> Deploying dotfiles...")
		if err := ssh.PipeFile(host, dotfilesTar, "tar xzf - -C $HOME"); err != nil {
			return fmt.Errorf("dotfiles: %w", err)
		}
	}

	// Deploy nvim plugins (if built)
	nvimTar := filepath.Join(scriptDir, "devlayer-nvim-plugins.tar.gz")
	if _, err := os.Stat(nvimTar); err == nil {
		fmt.Println("==> Deploying nvim plugins...")
		if err := ssh.PipeFile(host, nvimTar, "mkdir -p ~/.local/share/nvim && tar xzf - -C ~/.local/share/nvim"); err != nil {
			return fmt.Errorf("nvim plugins: %w", err)
		}
	}

	// Ensure PATH and env are configured (idempotent)
	hasProfile, _ := ssh.Run(host, `grep -q "devlayer managed PATH" ~/.profile 2>/dev/null && echo yes || echo no`)
	if hasProfile != "yes" {
		profileBlock := fmt.Sprintf(`
# devlayer managed PATH
export DEVLAYER_PREFIX="${DEVLAYER_PREFIX:-%s}"
export PATH="$DEVLAYER_PREFIX/bin:$PATH"
# SSL certs — pick the first existing cert bundle
for _cert in /etc/ssl/certs/ca-certificates.crt /etc/ssl/cert.pem /etc/pki/tls/certs/ca-bundle.crt; do
  [ -f "$_cert" ] && export GIT_SSL_CAINFO="$_cert" && break
done
unset _cert
`, remotePrefix)
		if err := ssh.PipeBytes(host, []byte(profileBlock), "cat >> ~/.profile"); err != nil {
			return fmt.Errorf("profile update: %w", err)
		}
	}

	fmt.Println()
	if err := Status(host); err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("==> Deploy complete. Start a new shell or: source ~/.profile")
	fmt.Printf("    prefix: %s\n", remotePrefix)
	return nil
}
