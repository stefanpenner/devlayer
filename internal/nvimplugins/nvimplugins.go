package nvimplugins

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// Plugin represents one entry from nvim-pack-lock.json.
type Plugin struct {
	Name string
	Rev  string `json:"rev"`
	Src  string `json:"src"`
}

// ParseLockfile reads nvim-pack-lock.json and returns plugin entries.
func ParseLockfile(path string) ([]Plugin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// vim.pack lockfile format: {"plugins": {"name": {"rev": "...", "src": "..."}}}
	var lockfile struct {
		Plugins map[string]Plugin `json:"plugins"`
	}
	if err := json.Unmarshal(data, &lockfile); err != nil {
		return nil, fmt.Errorf("parse nvim-pack-lock.json: %w", err)
	}

	plugins := make([]Plugin, 0, len(lockfile.Plugins))
	for name, p := range lockfile.Plugins {
		p.Name = name
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// LockfilePath returns the default nvim-pack-lock.json location.
func LockfilePath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "nvim", "nvim-pack-lock.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "nvim", "nvim-pack-lock.json")
}

// LocalPluginDir returns the default vim.pack plugin install directory.
func LocalPluginDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "nvim", "site", "pack", "core", "opt")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "nvim", "site", "pack", "core", "opt")
}

// pluginFetcher clones a missing plugin at its lockfile rev into dest.
type pluginFetcher func(p Plugin, dest string) error

// SyncPlugins copies locally-installed plugins into destDir, fetching any
// that are missing from the lockfile src at the pinned rev.
func SyncPlugins(lockfilePath, localPluginDir, destDir string) error {
	return syncPlugins(lockfilePath, localPluginDir, destDir, gitFetchPlugin)
}

func syncPlugins(lockfilePath, localPluginDir, destDir string, fetch pluginFetcher) error {
	plugins, err := ParseLockfile(lockfilePath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	for _, p := range plugins {
		dst := filepath.Join(destDir, p.Name)
		srcDir := filepath.Join(localPluginDir, p.Name)
		if _, err := os.Stat(srcDir); err == nil {
			if err := copyDirNoGit(srcDir, dst); err != nil {
				return fmt.Errorf("copy plugin %s: %w", p.Name, err)
			}
			continue
		}
		if fetch == nil {
			return fmt.Errorf("plugin %s not found locally and no fetcher", p.Name)
		}
		fmt.Printf("  fetch %s @ %s\n", p.Name, p.Rev)
		if err := fetch(p, dst); err != nil {
			return fmt.Errorf("plugin %s: %w", p.Name, err)
		}
	}
	return nil
}

func gitFetchPlugin(p Plugin, dest string) error {
	if p.Src == "" {
		return fmt.Errorf("empty src")
	}
	clone := exec.Command("git", "clone", "--filter=blob:none", p.Src, dest)
	clone.Stdout = os.Stderr
	clone.Stderr = os.Stderr
	if err := clone.Run(); err != nil {
		return fmt.Errorf("clone %s: %w", p.Src, err)
	}
	co := exec.Command("git", "-C", dest, "checkout", "--detach", p.Rev)
	co.Stdout = os.Stderr
	co.Stderr = os.Stderr
	if err := co.Run(); err != nil {
		os.RemoveAll(dest)
		return fmt.Errorf("checkout %s: %w", p.Rev, err)
	}
	return os.RemoveAll(filepath.Join(dest, ".git"))
}

// copyDirNoGit recursively copies src to dst, skipping .git directories.
func copyDirNoGit(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip .git directories
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		// Preserve symlinks
		if d.Type()&fs.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return nil // skip broken symlinks
			}
			return os.Symlink(linkTarget, target)
		}

		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
