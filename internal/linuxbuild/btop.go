package linuxbuild

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Btop fetches the tarball, cmake-builds static, copies the binary, and tars it.
func Btop(x Exec, out Output, env map[string]string, stdout io.Writer) error {
	ver, err := requireEnv(env, "BTOP_VERSION")
	if err != nil {
		return err
	}
	dir := "btop-" + ver

	if err := fetchTarball(x, "https://github.com/aristocratos/btop/archive/refs/tags/v"+ver+".tar.gz", dir+".tar.gz"); err != nil {
		return err
	}

	if err := x(dir, "cmake", "-B", "build",
		"-DCMAKE_BUILD_TYPE=Release",
		"-DBTOP_STATIC=ON",
		"-DBTOP_GPU=OFF",
		"-DBTOP_LTO=ON",
	); err != nil {
		return err
	}
	if err := x(dir, "cmake", "--build", "build", jobs()); err != nil {
		return err
	}

	bin, err := FindNamed(filepath.Join(dir, "build"), "btop")
	if err != nil {
		return err
	}
	tmp := os.TempDir()
	dest := filepath.Join(tmp, "btop")
	if err := copyFile(bin, dest); err != nil {
		return err
	}

	if err := x("", "strip", dest); err != nil {
		return err
	}
	return x("", "tar", "czf", "-", "-C", tmp, "btop")
}

// FindNamed returns the first regular file named name under root.
func FindNamed(root, name string) (string, error) {
	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if info.Name() != name {
			return nil
		}
		found = path
		return filepath.SkipAll
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("no %s under %s", name, root)
	}
	return found, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, st.Mode().Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}
