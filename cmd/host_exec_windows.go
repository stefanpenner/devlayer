//go:build windows

package cmd

import "fmt"

func execApp(path string, args []string) error {
	return fmt.Errorf("host: %s has no Windows build", path)
}
