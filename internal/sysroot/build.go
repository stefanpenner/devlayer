package sysroot

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Arch is a Linux musl target Zig can compile.
type Arch struct {
	Name   string // x86_64 or aarch64
	Triple string
}

func ArchByName(name string) (Arch, error) {
	switch name {
	case "x86_64":
		return Arch{Name: "x86_64", Triple: "x86_64-linux-musl"}, nil
	case "aarch64":
		return Arch{Name: "aarch64", Triple: "aarch64-linux-musl"}, nil
	default:
		return Arch{}, fmt.Errorf("arch must be x86_64 or aarch64, got %s", name)
	}
}

// Build compiles the libraries in dependency order and writes a sysroot
// directory plus a .tar.gz of that directory. out is the destination
// directory. The tarball path is returned.
func Build(archName, out string) (string, error) {
	arch, err := ArchByName(archName)
	if err != nil {
		return "", err
	}
	order, err := Order(packages())
	if err != nil {
		return "", err
	}
	zig, err := exec.LookPath("zig")
	if err != nil {
		return "", fmt.Errorf("zig is required to compile the sysroot: %w", err)
	}
	root, err := os.MkdirTemp("", "devlayer-sysroot-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(root)

	out, err = filepath.Abs(out)
	if err != nil {
		return "", err
	}
	prefix := filepath.Join(out, arch.Triple)
	if err := os.RemoveAll(prefix); err != nil {
		return "", err
	}
	if err := os.MkdirAll(prefix, 0755); err != nil {
		return "", err
	}
	b := &builder{arch: arch, zig: zig, root: root, prefix: prefix}
	if err := b.writeWrappers(); err != nil {
		return "", err
	}
	src := sources()
	for _, name := range order {
		fmt.Printf("==> %s %s\n", name, src[name].version)
		if err := b.one(name, src[name]); err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
	}
	if err := trim(prefix); err != nil {
		return "", err
	}
	if err := sanitizePC(prefix); err != nil {
		return "", err
	}
	tarPath := prefix + ".tar.gz"
	if err := writeTarGz(tarPath, prefix, arch.Triple); err != nil {
		return "", err
	}
	sum, err := fileSHA256(tarPath)
	if err != nil {
		return "", err
	}
	fmt.Printf("sysroot %s\nsha256 %s\n", tarPath, sum)
	return tarPath, nil
}

type builder struct {
	arch   Arch
	zig    string
	root   string
	prefix string
	cc     string
	cxx    string
	ar     string
	ranlib string
}

func (b *builder) writeWrappers() error {
	bin := filepath.Join(b.root, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		return err
	}
	b.cc = filepath.Join(bin, "cc")
	b.cxx = filepath.Join(bin, "c++")
	b.ar = filepath.Join(bin, "ar")
	b.ranlib = filepath.Join(bin, "ranlib")
	target := "-target " + b.arch.Triple
	scripts := map[string]string{
		b.cc:     "exec " + shellQuote(b.zig) + " cc " + target + ` "$@"`,
		b.cxx:    "exec " + shellQuote(b.zig) + " c++ " + target + ` "$@"`,
		b.ar:     "exec " + shellQuote(b.zig) + ` ar "$@"`,
		b.ranlib: "exec " + shellQuote(b.zig) + ` ranlib "$@"`,
	}
	for path, body := range scripts {
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0755); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) one(name string, src source) error {
	cache := filepath.Join(filepath.Dir(b.prefix), "cache")
	if err := os.MkdirAll(cache, 0755); err != nil {
		return err
	}
	tarball := filepath.Join(cache, filepath.Base(src.url))
	if err := download(src.url, tarball, src.sha256); err != nil {
		return err
	}
	dir := filepath.Join(b.root, "src", name)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := extract(tarball, dir); err != nil {
		return err
	}
	srcDir, err := singleChild(dir)
	if err != nil {
		return err
	}
	switch name {
	case "zlib":
		return b.cmake(srcDir, "-DBUILD_SHARED_LIBS=OFF")
	case "mbedtls":
		return b.cmake(srcDir,
			"-DENABLE_PROGRAMS=OFF",
			"-DENABLE_TESTING=OFF",
			"-DUSE_SHARED_MBEDTLS_LIBRARY=OFF",
			"-DUSE_STATIC_MBEDTLS_LIBRARY=ON",
		)
	case "ncurses":
		return b.ncurses(srcDir)
	case "curl":
		return b.cmake(srcDir,
			"-DBUILD_SHARED_LIBS=OFF",
			"-DBUILD_CURL_EXE=OFF",
			"-DCURL_USE_MBEDTLS=ON",
			"-DCURL_USE_OPENSSL=OFF",
			"-DCURL_USE_LIBPSL=OFF",
			"-DCURL_BROTLI=OFF",
			"-DCURL_ZSTD=OFF",
			"-DUSE_NGHTTP2=OFF",
			"-DUSE_LIBIDN2=OFF",
			"-DCURL_DISABLE_LDAP=ON",
			"-DHTTP_ONLY=ON",
			"-DCMAKE_PREFIX_PATH="+filepath.Join(b.prefix, "usr"),
		)
	default:
		return fmt.Errorf("no build step")
	}
}

func (b *builder) cmake(src string, extra ...string) error {
	build := filepath.Join(src, "build")
	args := []string{
		"-S", src, "-B", build,
		"-DCMAKE_BUILD_TYPE=MinSizeRel",
		"-DCMAKE_INSTALL_PREFIX=/usr",
		"-DCMAKE_C_COMPILER=" + b.cc,
		"-DCMAKE_CXX_COMPILER=" + b.cxx,
		"-DCMAKE_AR=" + b.ar,
		"-DCMAKE_RANLIB=" + b.ranlib,
		"-DCMAKE_SYSTEM_NAME=Linux",
		"-DCMAKE_FIND_ROOT_PATH=" + b.prefix,
		"-DCMAKE_FIND_ROOT_PATH_MODE_PROGRAM=NEVER",
		"-DCMAKE_FIND_ROOT_PATH_MODE_LIBRARY=ONLY",
		"-DCMAKE_FIND_ROOT_PATH_MODE_INCLUDE=ONLY",
	}
	args = append(args, extra...)
	if err := b.run(src, nil, "cmake", args...); err != nil {
		return err
	}
	if err := b.run(src, nil, "cmake", "--build", build, "--parallel"); err != nil {
		return err
	}
	return b.run(src, []string{"DESTDIR=" + b.prefix}, "cmake", "--install", build)
}

func (b *builder) ncurses(src string) error {
	// macOS /usr/bin/tic is too old for ncurses 6.5's terminfo.src.
	// Build tic with the host compiler, then let the cross build call it.
	tic, err := b.hostTic(src)
	if err != nil {
		return err
	}
	args := []string{
		"--host=" + b.arch.Triple,
		"--prefix=" + filepath.Join(b.prefix, "usr"),
		"--disable-shared",
		"--without-debug",
		"--without-cxx-binding",
		"--without-progs",
		"--without-tests",
		"--without-manpages",
		"--enable-widec",
		"--enable-overwrite",
		"--disable-db-install",
		"--with-fallbacks=xterm-256color,xterm,screen,tmux-256color,linux",
	}
	// tic and make_keys must run on this machine. CC is the Linux cross
	// compiler, so those two programs need the host compiler.
	hostCC := []string{"BUILD_CC=clang", "PATH=" + filepath.Dir(tic) + string(os.PathListSeparator) + "/usr/bin:/bin"}
	if err := b.run(src, hostCC, "./configure", args...); err != nil {
		return err
	}
	// tic writes the terminfo database. Parallel make races those writes.
	if err := b.run(src, hostCC, "make"); err != nil {
		return err
	}
	return b.run(src, hostCC, "make", "install")
}

func (b *builder) hostTic(src string) (string, error) {
	host := filepath.Join(b.root, "ncurses-host")
	if err := b.run(b.root, nil, "cp", "-a", src, host); err != nil {
		return "", err
	}
	hostEnv := []string{"CC=clang", "CXX=clang++", "BUILD_CC=clang"}
	if err := b.run(host, hostEnv, "./configure",
		"--prefix="+filepath.Join(b.root, "ncurses-host-prefix"),
		"--without-shared",
		"--without-cxx-binding",
		"--without-debug",
		"--without-tests",
		"--without-manpages",
	); err != nil {
		return "", err
	}
	for _, dir := range []string{"include", "ncurses"} {
		if err := b.run(host, hostEnv, "make", "-C", dir); err != nil {
			return "", err
		}
	}
	if err := b.run(host, hostEnv, "make", "-C", "progs", "tic"); err != nil {
		return "", err
	}
	return filepath.Join(host, "progs", "tic"), nil
}

func (b *builder) run(dir string, extra []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	// A clean environment. The host TERMINFO (Ghostty, on this machine)
	// otherwise becomes ncurses' terminfo directory and tic fails.
	env := map[string]string{
		"PATH":                   filepath.Dir(b.cc) + string(os.PathListSeparator) + "/usr/bin:/bin:/opt/homebrew/bin:" + filepath.Dir(b.zig),
		"HOME":                   os.Getenv("HOME"),
		"TMPDIR":                 os.Getenv("TMPDIR"),
		"CC":                     b.cc,
		"CXX":                    b.cxx,
		"AR":                     b.ar,
		"RANLIB":                 b.ranlib,
		"PKG_CONFIG_LIBDIR":      filepath.Join(b.prefix, "usr", "lib", "pkgconfig"),
		"PKG_CONFIG_SYSROOT_DIR": b.prefix,
	}
	for _, e := range extra {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			env[k] = v
		}
	}
	cmd.Env = make([]string, 0, len(env))
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func download(url, dest, want string) error {
	if got, err := fileSHA256(dest); err == nil && got == want {
		return nil
	}
	resp, err := httpGet(url)
	if err != nil {
		return err
	}
	defer resp.Close()
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp); err != nil {
		return err
	}
	got, err := fileSHA256(dest)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("sha256 %s, want %s", got, want)
	}
	return nil
}

func extract(tarball, dest string) error {
	f, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer f.Close()
	var r io.Reader = f
	switch {
	case strings.HasSuffix(tarball, ".tar.gz"), strings.HasSuffix(tarball, ".tgz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		r = gz
	case strings.HasSuffix(tarball, ".tar.bz2"):
		r = bzip2.NewReader(f)
	default:
		return fmt.Errorf("unknown archive %s", tarball)
	}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if name == "." || strings.HasPrefix(name, "..") {
			continue
		}
		path := filepath.Join(dest, name)
		if !strings.HasPrefix(path, dest+string(os.PathSeparator)) && path != dest {
			return fmt.Errorf("path escapes archive: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			mode := hdr.FileInfo().Mode().Perm()
			if mode == 0 {
				mode = 0644
			}
			out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
}

func singleChild(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) != 1 {
		return "", fmt.Errorf("%s has %d top-level directories", dir, len(dirs))
	}
	return filepath.Join(dir, dirs[0]), nil
}

func sanitizePC(prefix string) error {
	dir := filepath.Join(prefix, "usr", "lib", "pkgconfig")
	abs := filepath.Join(prefix, "usr", "lib")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".pc") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		next := strings.ReplaceAll(string(body), "-L"+abs, "-L${libdir}")
		if err := os.WriteFile(path, []byte(next), 0644); err != nil {
			return err
		}
	}
	return nil
}

func trim(prefix string) error {
	root := filepath.Join(prefix, "usr")
	for _, dir := range []string{"share", "bin", filepath.Join("lib", "cmake")} {
		if err := os.RemoveAll(filepath.Join(root, dir)); err != nil {
			return err
		}
	}
	return filepath.Walk(filepath.Join(root, "lib"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.Contains(info.Name(), ".so") {
			return os.Remove(path)
		}
		return nil
	})
}

func writeTarGz(dest, prefix, triple string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	return filepath.Walk(prefix, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(prefix, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(triple, rel))
		if info.IsDir() {
			return tw.WriteHeader(&tar.Header{Name: name + "/", Mode: 0755, Typeflag: tar.TypeDir})
		}
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Name = name
		hdr.ModTime = time.Unix(0, 0)
		hdr.AccessTime = hdr.ModTime
		hdr.ChangeTime = hdr.ModTime
		hdr.Uname = ""
		hdr.Gname = ""
		hdr.Uid = 0
		hdr.Gid = 0
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(tw, in)
		return err
	})
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
