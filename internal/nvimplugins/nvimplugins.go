// Package nvimplugins installs vim.pack plugins at the lockfile commit.
//
// parse lock → fetch each GitHub archive at rev
package nvimplugins

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/stefanpenner/devlayer/internal/download"
)

// Plugin represents one entry from nvim-pack-lock.json.
type Plugin struct {
	Name string
	Rev  string `json:"rev"`
	Src  string `json:"src"`
}

var commitRev = regexp.MustCompile(`^[0-9a-f]{40}$`)

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

// LockfilePath returns the installed nvim-pack-lock.json (devlayer ls).
func LockfilePath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "nvim", "nvim-pack-lock.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "nvim", "nvim-pack-lock.json")
}

// RepoLockfile is the lockfile shipped in the devlayer repo.
func RepoLockfile(root string) string {
	return filepath.Join(root, "config", "nvim", "nvim-pack-lock.json")
}

// LocalPluginDir returns the default vim.pack plugin install directory.
func LocalPluginDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "nvim", "site", "pack", "core", "opt")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "nvim", "site", "pack", "core", "opt")
}

// ArchiveURL is the GitHub archive for the lockfile commit.
// The rev is the pin. GitHub archive bytes are not stable, so this is not a sha256.
func ArchiveURL(p Plugin) (string, error) {
	if !commitRev.MatchString(p.Rev) {
		return "", fmt.Errorf("plugin %s: rev %q is not a full commit", p.Name, p.Rev)
	}
	owner, repo, err := githubRepo(p.Src)
	if err != nil {
		return "", fmt.Errorf("plugin %s: %w", p.Name, err)
	}
	return fmt.Sprintf("https://github.com/%s/%s/archive/%s.tar.gz", owner, repo, p.Rev), nil
}

func githubRepo(src string) (string, string, error) {
	u, err := url.Parse(src)
	if err != nil {
		return "", "", err
	}
	host := strings.ToLower(u.Host)
	if host != "github.com" && host != "www.github.com" {
		return "", "", fmt.Errorf("src host %q", u.Host)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("src path %q", u.Path)
	}
	repo := strings.TrimSuffix(parts[1], ".git")
	if repo == "" {
		return "", "", fmt.Errorf("empty repo")
	}
	return parts[0], repo, nil
}

// pluginFetcher installs one plugin into dest.
type pluginFetcher func(p Plugin, dest string) error

// SyncPlugins fetches every lockfile plugin at its pinned rev into destDir.
// A local vim.pack tree is not consulted.
func SyncPlugins(lockfilePath, destDir string) error {
	return syncPlugins(lockfilePath, destDir, fetchArchive)
}

func syncPlugins(lockfilePath, destDir string, fetch pluginFetcher) error {
	plugins, err := ParseLockfile(lockfilePath)
	if err != nil {
		return err
	}
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Name < plugins[j].Name })

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	if fetch == nil {
		return fmt.Errorf("plugin fetcher is nil")
	}

	for _, p := range plugins {
		dst := filepath.Join(destDir, p.Name)
		fmt.Printf("  fetch %s @ %s\n", p.Name, p.Rev)
		if err := fetch(p, dst); err != nil {
			return fmt.Errorf("plugin %s: %w", p.Name, err)
		}
	}
	return nil
}

// httpGet is the archive GET. Tests replace it.
var httpGet = http.Get

func fetchArchive(p Plugin, dest string) error {
	archiveURL, err := ArchiveURL(p)
	if err != nil {
		return err
	}
	tmp, err := httpTemp(archiveURL)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	if err := download.UnpackTarGz(tmp, dest, 1); err != nil {
		os.RemoveAll(dest)
		return err
	}
	return nil
}

func httpTemp(rawURL string) (string, error) {
	resp, err := httpGet(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %d", rawURL, resp.StatusCode)
	}
	f, err := os.CreateTemp("", "devlayer-plugin-*.tar.gz")
	if err != nil {
		return "", err
	}
	name := f.Name()
	_, err = io.Copy(f, resp.Body)
	closeErr := f.Close()
	if err != nil {
		os.Remove(name)
		return "", err
	}
	if closeErr != nil {
		os.Remove(name)
		return "", closeErr
	}
	return name, nil
}
