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

// Install extracts a devlayer bundle to the local DEVLAYER_PREFIX.
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
	// Re-sign Mach-O binaries on macOS (extraction invalidates adhoc signatures)
	if runtime.GOOS == "darwin" {
		binDir := filepath.Join(prefix, "bin")
		nvimBin := filepath.Join(prefix, "nvim", "bin", "nvim")
		for _, bin := range []string{nvimBin, filepath.Join(binDir, "nvim")} {
			if info, err := os.Lstat(bin); err == nil && info.Mode().IsRegular() {
				exec.Command("codesign", "--force", "--sign", "-", bin).Run()
			}
		}
	}

	// Install dotfiles (if built)
	dotfilesTar := filepath.Join(scriptDir, "devlayer-dotfiles.tar.gz")
	if _, err := os.Stat(dotfilesTar); err == nil {
		home, _ := os.UserHomeDir()
		fmt.Println("==> Installing dotfiles...")
		if err := archive.ExtractTarGz(dotfilesTar, home); err != nil {
			return fmt.Errorf("dotfiles: %w", err)
		}
	}

	// Install nvim plugins (if built)
	nvimTar := filepath.Join(scriptDir, "devlayer-nvim-plugins.tar.gz")
	if _, err := os.Stat(nvimTar); err == nil {
		home, _ := os.UserHomeDir()
		nvimDataDir := filepath.Join(home, ".local", "share", "nvim")
		fmt.Println("==> Installing nvim plugins...")
		if err := os.MkdirAll(nvimDataDir, 0755); err != nil {
			return err
		}
		if err := archive.ExtractTarGz(nvimTar, nvimDataDir); err != nil {
			return fmt.Errorf("nvim plugins: %w", err)
		}
	}

	fmt.Printf("==> Done. Ensure PATH includes %s%cbin\n", prefix, filepath.Separator)
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

// findBundle looks in cwd first, then extraDir (docker/source context).
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
	return "", fmt.Errorf("no bundle found at %s", name)
}
