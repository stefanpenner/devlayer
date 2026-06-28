package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed assets/eza-carbonfox.yml
var ezaCarbonfoxTheme []byte

func installEzaTheme(outDir string) error {
	fmt.Println("  eza carbonfox theme")
	ezaConfigDir := filepath.Join(outDir, "share", "eza")
	if err := os.MkdirAll(ezaConfigDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ezaConfigDir, "theme.yml"), ezaCarbonfoxTheme, 0644)
}
