package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/stefanpenner/devlayer/internal/config"
)

const defaultConfig = `# Paths relative to $HOME. Config only — never private keys.
[dotfiles]
sync = [
  # shell
  ".bash_profile",
  ".zshrc",
  ".config/zsh",
  ".p10k.zsh",

  # terminal + editor
  ".config/ghostty",
  ".config/nvim",
  ".config/opencode",
  ".gitconfig",

  # optional SSH layer: config only, never private keys
  ".ssh/config",
  ".ssh/config.d",
]
`

// Init writes ~/.config/devlayer/config.toml from the default template.
func Init(force bool) error {
	path := config.Path()
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("%s already exists (pass --force to overwrite)", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(defaultConfig), 0644); err != nil {
		return err
	}
	fmt.Printf("==> wrote %s\n", path)
	fmt.Println("Edit sync paths, then: devlayer build && devlayer install")
	return nil
}
