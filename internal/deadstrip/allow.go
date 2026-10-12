package deadstrip

import (
	"os"
	"path/filepath"

	"github.com/stefanpenner/devlayer/internal/loadcheck"
)

// Check fails when a 64-bit Mach-O file fails its policy.
// bininfo --policy is the check. This function calls it.
func Check(in []byte) error {
	if !isMach64(in) {
		return nil
	}
	dir, err := os.MkdirTemp("", "deadstrip-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, in, 0o644); err != nil {
		return err
	}
	return loadcheck.File(path)
}

// CheckFile runs the policy for one path.
func CheckFile(path string) error {
	return loadcheck.File(path)
}

// CheckTree runs the policy for each file under root.
func CheckTree(root string) error {
	return loadcheck.Tree(root)
}
