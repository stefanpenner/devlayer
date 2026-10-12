package assemble

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Input is pinned bytes already on disk.
// Place copies them. It does not download.
type Input struct {
	Bins  []Bin
	Trees []Tree
	Files []File
}

// Bin is one file copied to bin/<name>.
type Bin struct {
	Name string
	Path string
}

// Tree is one directory copied to Dest.
type Tree struct {
	Dest string
	Path string
}

// File is one file copied to Dest.
type File struct {
	Dest string
	Path string
}

// ParseNamed splits one name=path flag.
func ParseNamed(spec string) (string, string, error) {
	name, path, ok := strings.Cut(spec, "=")
	if !ok || name == "" || path == "" {
		return "", "", fmt.Errorf("assemble: want name=path, got %q", spec)
	}
	return name, path, nil
}

// Place copies bins, trees, and files into staging.
func Place(staging string, in Input) error {
	if err := os.MkdirAll(filepath.Join(staging, "bin"), 0755); err != nil {
		return err
	}
	for _, bin := range in.Bins {
		dest, err := destPath(staging, filepath.Join("bin", bin.Name))
		if err != nil {
			return err
		}
		if err := copyFileMode(bin.Path, dest, 0755); err != nil {
			return fmt.Errorf("bin %s: %w", bin.Name, err)
		}
	}
	for _, tree := range in.Trees {
		dest, err := destPath(staging, tree.Dest)
		if err != nil {
			return err
		}
		if err := copyTree(tree.Path, dest); err != nil {
			return fmt.Errorf("tree %s: %w", tree.Dest, err)
		}
	}
	for _, file := range in.Files {
		dest, err := destPath(staging, file.Dest)
		if err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if st, err := os.Stat(file.Path); err == nil && st.Mode()&0111 != 0 {
			mode = 0755
		}
		if err := copyFileMode(file.Path, dest, mode); err != nil {
			return fmt.Errorf("file %s: %w", file.Dest, err)
		}
	}
	return nil
}

func destPath(staging, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("assemble: bad dest %q", rel)
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("assemble: dest escapes: %q", rel)
	}
	return filepath.Join(staging, clean), nil
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0755)
		}
		if skipRootMeta(rel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		out := filepath.Join(dst, rel)
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
				return err
			}
			// A link into the Bazel cache is not portable. Copy the file.
			if !linkStaysInside(src, path, target) {
				followed, err := os.Stat(path)
				if err != nil {
					return err
				}
				return copyFileMode(path, out, followed.Mode().Perm())
			}
			return os.Symlink(target, out)
		}
		if info.IsDir() {
			return os.MkdirAll(out, 0755)
		}
		return copyFileMode(path, out, info.Mode().Perm())
	})
}

func linkStaysInside(root, linkPath, target string) bool {
	var dest string
	if filepath.IsAbs(target) {
		dest = target
	} else {
		dest = filepath.Join(filepath.Dir(linkPath), target)
	}
	rel, err := filepath.Rel(root, dest)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func skipRootMeta(rel string) bool {
	if strings.Contains(rel, string(filepath.Separator)) {
		return false
	}
	switch rel {
	case "BUILD", "BUILD.bazel", "WORKSPACE", "WORKSPACE.bazel", "MODULE.bazel", "REPO.bazel":
		return true
	default:
		return false
	}
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}
