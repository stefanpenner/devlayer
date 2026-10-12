//go:build unix

package cmd

import (
	"os"
	"syscall"
)

func execApp(path string, args []string) error {
	argv := append([]string{path}, args...)
	return syscall.Exec(path, argv, os.Environ())
}
