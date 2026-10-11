package deadstrip

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// File rewrites one Mach-O when Strip changes it.
// Any other file stays on disk.
func File(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	var mag [4]byte
	n, err := io.ReadFull(f, mag[:])
	f.Close()
	if err == io.EOF || err == io.ErrUnexpectedEOF || n < 4 {
		return nil
	}
	if err != nil {
		return err
	}
	if !isMach64(mag[:]) {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	in, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, changed, err := Strip(in)
	if err != nil || !changed {
		return err
	}
	if err := os.WriteFile(path, out, info.Mode().Perm()); err != nil {
		return err
	}
	return Sign(path)
}

// Sign replaces the code signature after a load command change.
// macOS kills an arm64 program whose signature no longer matches.
func Sign(path string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	bin, err := exec.LookPath("codesign")
	if err != nil {
		return errors.New("deadstrip: codesign is missing")
	}
	cmd := exec.Command(bin, "--force", "--sign", "-", path)
	msg, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("deadstrip: codesign: %s", bytes.TrimSpace(msg))
	}
	return nil
}

// Tree runs File on each regular file under root.
func Tree(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		return File(path)
	})
}
