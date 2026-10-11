package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds the devlayer configuration.
type Config struct {
	Dotfiles DotfilesConfig `toml:"dotfiles"`
}

// DotfilesConfig lists paths (relative to $HOME) to sync.
type DotfilesConfig struct {
	Sync []string `toml:"sync"`
}

// Load reads the config file, returning empty defaults if it doesn't exist.
func Load() (*Config, error) {
	cfg := &Config{}
	path := Path()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Path returns the config file location.
func Path() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "devlayer", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "devlayer", "config.toml")
}

// PrivateConfig is the optional house layer. It lives beside config.toml
// and is never part of the public dotfiles sync list.
type PrivateConfig struct {
	Repo string `toml:"repo"`
	Path string `toml:"path"`
}

// LoadPrivate reads private.toml. A missing file is not an error.
func LoadPrivate() (*PrivateConfig, error) {
	file := privatePath()
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var raw struct {
		Private PrivateConfig `toml:"private"`
	}
	if _, err := toml.DecodeFile(file, &raw); err != nil {
		return nil, err
	}
	if raw.Private.Repo == "" || raw.Private.Path == "" {
		return nil, fmt.Errorf("%s needs repo and path", file)
	}
	expanded, err := expandHome(raw.Private.Path)
	if err != nil {
		return nil, err
	}
	raw.Private.Path = expanded
	return &raw.Private, nil
}

func privatePath() string {
	return filepath.Join(filepath.Dir(Path()), "private.toml")
}

func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}
