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

	arch, err := remoteArch(host)
	if err != nil {
		return err
	}
	tarball, err := ensureLinuxBundle(arch, scriptDir, vers)
	if err != nil {
		return err
	}

	prefix := remotePrefix(host)
	fmt.Printf("==> Deploying to %s (%s)...\n", host, prefix)
	if err := pipeBundle(host, prefix, tarball); err != nil {
		return err
	}
	if err := deployDotfiles(host, scriptDir); err != nil {
		return err
	}
	if err := deployNvimPlugins(host, scriptDir); err != nil {
		return err
	}
	if err := ensureRemoteProfile(host, prefix); err != nil {
		return err
	}

	fmt.Println()
	if err := Status(host); err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("==> Deploy complete. Start a new shell or: source ~/.profile")
	fmt.Printf("    prefix: %s\n", prefix)
	return nil
}

func remoteArch(host string) (string, error) {
	arch, err := ssh.Run(host, "uname -m")
	if err != nil {
		return "", fmt.Errorf("failed to detect remote arch: %w", err)
	}
	return arch, nil
}

func ensureLinuxBundle(arch, scriptDir string, vers *versions.Versions) (string, error) {
	bundleName := fmt.Sprintf("devlayer-linux-%s.tar.gz", arch)
	tarball, err := findBundle(bundleName, scriptDir)
	if err == nil {
		return tarball, nil
	}

	fmt.Printf("==> No bundle found, building for linux/%s...\n", arch)
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return "", cwdErr
	}
	if err := Build([]string{"--os", "linux", "--arch", arch}, vers, scriptDir, cwd); err != nil {
		return "", fmt.Errorf("auto-build failed: %w", err)
	}
	return findBundle(bundleName, scriptDir)
}

func remotePrefix(host string) string {
	prefix, err := ssh.Run(host, `echo "${DEVLAYER_PREFIX:-$HOME/.local}"`)
	if err != nil {
		return "$HOME/.local"
	}
	return prefix
}

func pipeBundle(host, prefix, tarball string) error {
	if err := ssh.RunInteractive(host, fmt.Sprintf("mkdir -p '%s/bin'", prefix)); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := ssh.PipeFile(host, tarball, fmt.Sprintf("tar xzf - -C '%s'", prefix)); err != nil {
		return fmt.Errorf("tar extract: %w", err)
	}
	return nil
}

func deployDotfiles(host, scriptDir string) error {
	dotfilesTar := filepath.Join(scriptDir, "devlayer-dotfiles.tar.gz")
	if _, err := os.Stat(dotfilesTar); err != nil {
		return nil
	}
	fmt.Println("==> Deploying dotfiles...")
	if err := ssh.PipeFile(host, dotfilesTar, "tar xzf - -C $HOME"); err != nil {
		return fmt.Errorf("dotfiles: %w", err)
	}
	return nil
}

func deployNvimPlugins(host, scriptDir string) error {
	nvimTar := filepath.Join(scriptDir, "devlayer-nvim-plugins.tar.gz")
	if _, err := os.Stat(nvimTar); err != nil {
		return nil
	}
	fmt.Println("==> Deploying nvim plugins...")
	if err := ssh.PipeFile(host, nvimTar, "mkdir -p ~/.local/share/nvim && tar xzf - -C ~/.local/share/nvim"); err != nil {
		return fmt.Errorf("nvim plugins: %w", err)
	}
	return nil
}

func ensureRemoteProfile(host, prefix string) error {
	hasProfile, _ := ssh.Run(host, `grep -q "devlayer managed PATH" ~/.profile 2>/dev/null && echo yes || echo no`)
	if hasProfile == "yes" {
		return nil
	}
	profileBlock := fmt.Sprintf(`
# devlayer managed PATH
export DEVLAYER_PREFIX="${DEVLAYER_PREFIX:-%s}"
export PATH="$DEVLAYER_PREFIX/bin:$PATH"
# SSL certs — pick the first existing cert bundle
for _cert in /etc/ssl/certs/ca-certificates.crt /etc/ssl/cert.pem /etc/pki/tls/certs/ca-bundle.crt; do
  [ -f "$_cert" ] && export GIT_SSL_CAINFO="$_cert" && break
done
unset _cert
`, prefix)
	if err := ssh.PipeBytes(host, []byte(profileBlock), "cat >> ~/.profile"); err != nil {
		return fmt.Errorf("profile update: %w", err)
	}
	return nil
}
