package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"os/exec"

	"github.com/stefanpenner/devlayer/internal/archive"
	"github.com/stefanpenner/devlayer/internal/platform"
)

// Install extracts the sysroot archive into DEVLAYER_PREFIX.
func Install(scriptDir string) error {
	p, err := platform.New(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	arch := bundleArch(p)
	bundleName := fmt.Sprintf("devlayer-%s-%s.%s", p.OS, arch, p.BundleExt)
	bundle, err := findBundle(bundleName, scriptDir)
	if err != nil {
		return fmt.Errorf("%w\nRun: devlayer build --os %s --arch %s", err, p.OS, arch)
	}

	prefix := defaultPrefix()

	fmt.Printf("==> Installing to %s...\n", prefix)
	if err := os.MkdirAll(prefix, 0755); err != nil {
		return err
	}

	if p.BundleExt == "zip" {
		err = archive.ExtractZip(bundle, prefix)
	} else {
		err = archive.ExtractTarGz(bundle, prefix)
	}
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	resignDarwin(prefix)
	if err := installDotfiles(scriptDir); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if err := installPrivate(scriptDir, home); err != nil {
		return err
	}
	if err := installNvimPlugins(scriptDir); err != nil {
		return err
	}
	if err := placeNvimConfig(scriptDir, prefix); err != nil {
		return err
	}
	if err := placeNvimData(scriptDir, prefix); err != nil {
		return err
	}

	fmt.Printf("==> Done. Ensure PATH includes %s%cbin\n", prefix, filepath.Separator)
	return nil
}

// resignDarwin signs the two nvim binaries again.
// Extract clears an ad-hoc Mach-O signature.
func resignDarwin(prefix string) {
	if runtime.GOOS != "darwin" {
		return
	}
	bins := []string{
		filepath.Join(prefix, "nvim", "bin", "nvim"),
		filepath.Join(prefix, "bin", "nvim"),
	}
	for _, bin := range bins {
		info, err := os.Lstat(bin)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		exec.Command("codesign", "--force", "--sign", "-", bin).Run()
	}
}

// installDotfiles extracts the dotfile archive into the home directory.
// A missing archive is not an error.
func installDotfiles(scriptDir string) error {
	tar := filepath.Join(scriptDir, "devlayer-dotfiles.tar.gz")
	if _, err := os.Stat(tar); err != nil {
		return nil
	}
	home, _ := os.UserHomeDir()
	fmt.Println("==> Installing dotfiles...")
	if err := archive.ExtractTarGz(tar, home); err != nil {
		return fmt.Errorf("dotfiles: %w", err)
	}
	return nil
}

// placeNvimConfig copies the repo nvim config into the sysroot.
// A checkout without init.lua skips the copy.
func placeNvimConfig(root, prefix string) error {
	src := filepath.Join(root, "config", "nvim")
	if _, err := os.Stat(filepath.Join(src, "init.lua")); err != nil {
		return nil
	}
	dst := filepath.Join(prefix, "share", "devlayer", "config", "nvim")
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return copyDirSimple(src, dst)
}

// placeNvimData extracts the plugin archive into the sysroot data tree.
// A missing archive is not an error.
func placeNvimData(scriptDir, prefix string) error {
	tar := filepath.Join(scriptDir, "devlayer-nvim-plugins.tar.gz")
	if _, err := os.Stat(tar); err != nil {
		return nil
	}
	dir := filepath.Join(prefix, "share", "devlayer", "data", "nvim")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return archive.ExtractTarGz(tar, dir)
}

// installNvimPlugins extracts the plugin archive into the nvim data directory.
// A missing archive is not an error.
func installNvimPlugins(scriptDir string) error {
	tar := filepath.Join(scriptDir, "devlayer-nvim-plugins.tar.gz")
	if _, err := os.Stat(tar); err != nil {
		return nil
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".local", "share", "nvim")
	fmt.Println("==> Installing nvim plugins...")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := archive.ExtractTarGz(tar, dir); err != nil {
		return fmt.Errorf("nvim plugins: %w", err)
	}
	return nil
}

// Linux bundles use the Rust arch (aarch64). Darwin and Windows use ArchGeneric (arm64).
func bundleArch(p *platform.Platform) string {
	if p.OS == "linux" {
		return p.RustArch
	}
	return p.ArchGeneric
}

// defaultPrefix returns the install location from DEVLAYER_PREFIX or the platform default.
func defaultPrefix() string {
	if p := os.Getenv("DEVLAYER_PREFIX"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "devlayer")
		}
		return filepath.Join(home, "AppData", "Local", "devlayer")
	}
	return filepath.Join(home, ".local")
}

// findBundle looks in cwd first, then extraDir.
func findBundle(name, extraDir string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	for _, dir := range []string{cwd, extraDir} {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no sysroot archive found at %s", name)
}
