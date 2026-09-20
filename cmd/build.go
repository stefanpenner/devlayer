package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/stefanpenner/devlayer/internal/archive"
	"github.com/stefanpenner/devlayer/internal/binaries"
	"github.com/stefanpenner/devlayer/internal/config"
	"github.com/stefanpenner/devlayer/internal/docker"
	"github.com/stefanpenner/devlayer/internal/download"
	"github.com/stefanpenner/devlayer/internal/nvimplugins"
	"github.com/stefanpenner/devlayer/internal/platform"
	"github.com/stefanpenner/devlayer/internal/versions"
)

// buildOptions holds flags parsed from the build command line.
type buildOptions struct {
	os       string
	arch     string
	nvimHead bool // build nvim from HEAD instead of stable/pinned version
}

func parseBuildOpts(args []string, defaultOS, defaultArch string) (buildOptions, error) {
	opts := buildOptions{
		os:   defaultOS,
		arch: defaultArch,
	}
	switch opts.arch {
	case "amd64":
		opts.arch = "x86_64"
	case "arm64":
		opts.arch = "aarch64"
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--os":
			i++
			if i < len(args) {
				opts.os = args[i]
			}
		case "--arch":
			i++
			if i < len(args) {
				opts.arch = args[i]
			}
		case "--nvim-head":
			opts.nvimHead = true
		default:
			return opts, fmt.Errorf("unknown option: %s", args[i])
		}
	}
	return opts, nil
}

// Build builds a devlayer bundle for the given OS and architecture.
// Artifacts are written to outDir (cwd). scriptDir is the docker/source context.
func Build(args []string, vers *versions.Versions, scriptDir, outDir string) error {
	opts, err := parseBuildOpts(args, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	switch opts.os {
	case "darwin":
		if err := buildDarwin(opts, vers, outDir); err != nil {
			return err
		}
	case "windows":
		if err := buildWindows(opts.arch, vers, outDir); err != nil {
			return err
		}
	default:
		if err := buildLinux(opts.arch, scriptDir, outDir, vers); err != nil {
			return err
		}
	}

	if err := buildDotfiles(outDir); err != nil {
		return err
	}
	if err := buildNvimPlugins(outDir); err != nil {
		return err
	}

	return nil
}

func buildDarwin(opts buildOptions, vers *versions.Versions, scriptDir string) error {
	p, err := platform.New("darwin", opts.arch)
	if err != nil {
		return err
	}

	// Use ArchGeneric for display and filenames (arm64, not aarch64, for darwin)
	displayArch := p.ArchGeneric
	fmt.Printf("==> Building devlayer for darwin/%s...\n", displayArch)

	out, err := os.MkdirTemp("", "devlayer-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(out)

	binDir := filepath.Join(out, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	fmt.Println("==> Downloading binaries (darwin/" + displayArch + ")")

	// Skip nvim download — we build it from source
	if err := downloadAll(out, p, vers, map[string]bool{"nvim": true}); err != nil {
		return err
	}

	// Build tools from source for best portability
	fmt.Println("==> Building tools from source...")
	if err := buildNvim(out, vers, opts.nvimHead); err != nil {
		return err
	}
	if err := buildHtop(binDir, vers); err != nil {
		return err
	}
	if err := buildBtop(binDir, vers); err != nil {
		return err
	}

	// Build eza from source (no pre-built macOS binary)
	libexecDir := filepath.Join(out, "libexec")
	if err := os.MkdirAll(libexecDir, 0755); err != nil {
		return fmt.Errorf("mkdir libexec: %w", err)
	}
	if err := buildEza(libexecDir, vers); err != nil {
		return err
	}

	// Build make from source
	if err := buildMake(binDir, vers); err != nil {
		return err
	}

	// Create wrapper scripts in bin/ for tools with runtime dependencies
	wrappers := []wrapper{
		{"nvim", "nvim/bin/nvim", map[string]string{"VIMRUNTIME": "$PREFIX/nvim/share/nvim/runtime"}},
		{"go", "go/bin/go", map[string]string{"GOROOT": "$PREFIX/go"}},
		{"gofmt", "go/bin/gofmt", map[string]string{"GOROOT": "$PREFIX/go"}},
		{"zig", "zig/zig", nil},
	}

	// Create eza wrapper that sets EZA_CONFIG_DIR for the bundled theme
	// Real binary lives in libexec/eza to avoid naming conflict with wrapper
	ezaWrapper := `#!/bin/sh
PREFIX="$(cd "$(dirname "$0")/.." && pwd)"
export EZA_CONFIG_DIR="$PREFIX/share/eza"
exec "$PREFIX/libexec/eza" "$@"
`
	if err := os.WriteFile(filepath.Join(binDir, "eza"), []byte(ezaWrapper), 0755); err != nil {
		return fmt.Errorf("wrapper eza: %w", err)
	}
	// Create ls alias that delegates to eza
	if err := os.WriteFile(filepath.Join(binDir, "ls"), []byte(ezaWrapper), 0755); err != nil {
		return fmt.Errorf("wrapper ls: %w", err)
	}
	if err := createUnixWrappers(binDir, wrappers); err != nil {
		return err
	}

	// Create cc/c++ convenience wrappers that delegate to zig
	if err := createZigCCWrappers(binDir); err != nil {
		return err
	}

	// Copy self into bundle
	if self, err := os.Executable(); err == nil {
		fmt.Println("  devlayer")
		if err := copyFile(self, filepath.Join(binDir, "devlayer")); err != nil {
			return fmt.Errorf("copy devlayer: %w", err)
		}
	}

	// Generate checksums
	if err := generateChecksums(out); err != nil {
		return err
	}

	outputFile := filepath.Join(scriptDir, fmt.Sprintf("devlayer-darwin-%s.tar.gz", displayArch))
	if err := archive.CreateTarGz(outputFile, out); err != nil {
		return err
	}

	docker.PrintSize(outputFile)
	fmt.Println("==> Build complete.")
	return nil
}

func buildWindows(arch string, vers *versions.Versions, scriptDir string) error {
	p, err := platform.New("windows", arch)
	if err != nil {
		return err
	}

	fmt.Printf("==> Building devlayer for windows/%s...\n", p.ArchGeneric)

	out, err := os.MkdirTemp("", "devlayer-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(out)

	binDir := filepath.Join(out, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	fmt.Println("==> Downloading binaries (windows/" + p.ArchGeneric + ")")

	if err := downloadAll(out, p, vers, nil); err != nil {
		return err
	}

	// Create .cmd wrapper scripts for Windows
	wrappers := []wrapper{
		{"git", `git\cmd\git.exe`, map[string]string{"GIT_EXEC_PATH": `%PREFIX%\git\mingw64\libexec\git-core`}},
		{"nvim", `nvim\bin\nvim.exe`, map[string]string{"VIMRUNTIME": `%PREFIX%\nvim\share\nvim\runtime`}},
		{"go", `go\bin\go.exe`, map[string]string{"GOROOT": `%PREFIX%\go`}},
		{"gofmt", `go\bin\gofmt.exe`, map[string]string{"GOROOT": `%PREFIX%\go`}},
		{"zig", `zig\zig.exe`, nil},
	}
	if err := createWindowsWrappers(binDir, wrappers); err != nil {
		return err
	}

	// Create cc/c++ wrappers for Windows
	if err := createZigCCWrappersWindows(binDir); err != nil {
		return err
	}

	// Copy self into bundle
	if self, err := os.Executable(); err == nil {
		fmt.Println("  devlayer")
		destName := "devlayer"
		if runtime.GOOS == "windows" {
			destName = "devlayer.exe"
		}
		if err := copyFile(self, filepath.Join(binDir, destName)); err != nil {
			return fmt.Errorf("copy devlayer: %w", err)
		}
	}

	// Generate checksums
	if err := generateChecksums(out); err != nil {
		return err
	}

	outputFile := filepath.Join(scriptDir, fmt.Sprintf("devlayer-windows-%s.zip", p.ArchGeneric))
	if err := archive.CreateZip(outputFile, out); err != nil {
		return err
	}

	docker.PrintSize(outputFile)
	fmt.Println("==> Build complete.")
	return nil
}

func buildLinux(arch, scriptDir, outDir string, vers *versions.Versions) error {
	if _, err := exec.LookPath("bazel"); err == nil {
		if _, err := os.Stat(filepath.Join(scriptDir, "MODULE.bazel")); err == nil {
			return bazelLinuxBundle(arch, scriptDir, outDir)
		}
	}
	return dockerLinuxBundle(arch, scriptDir, outDir, vers)
}

func bazelLinuxBundle(arch, scriptDir, outDir string) error {
	fmt.Printf("==> Building devlayer for linux/%s (bazel)...\n", arch)

	cmd := exec.Command("bazel", "build", "//linux:bundle_"+arch)
	cmd.Dir = scriptDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bazel build //linux:bundle_%s: %w", arch, err)
	}

	src := filepath.Join(scriptDir, "bazel-bin", "linux", fmt.Sprintf("devlayer-linux-%s.tar.gz", arch))
	dst := filepath.Join(outDir, fmt.Sprintf("devlayer-linux-%s.tar.gz", arch))
	if err := copyFile(src, dst); err != nil {
		return err
	}
	docker.PrintSize(dst)
	fmt.Println("==> Build complete.")
	return nil
}

func dockerLinuxBundle(arch, scriptDir, outDir string, vers *versions.Versions) error {
	fmt.Printf("==> Building devlayer for linux/%s (Docker)...\n", arch)

	if _, err := os.Stat(filepath.Join(scriptDir, "internal", "binaries")); err != nil {
		return fmt.Errorf("linux docker build needs a source checkout (internal/binaries missing); try bazel build //linux:bundle_%s", arch)
	}

	p, err := platform.New("linux", arch)
	if err != nil {
		return err
	}

	args := map[string]string{
		"GIT_VERSION":  vers.Get("GIT_VERSION"),
		"ZSH_VERSION":  vers.Get("ZSH_VERSION"),
		"HTOP_VERSION": vers.Get("HTOP_VERSION"),
		"BTOP_VERSION": vers.Get("BTOP_VERSION"),
		"NVIM_VERSION": vers.Get("NVIM_VERSION"),
		"MAKE_VERSION": vers.Get("MAKE_VERSION"),
	}
	if err := docker.Build(p.DockerPlatform, "devlayer", scriptDir, args); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}

	outputFile := filepath.Join(outDir, fmt.Sprintf("devlayer-linux-%s.tar.gz", arch))
	if err := docker.RunToFile("devlayer", outputFile); err != nil {
		return fmt.Errorf("docker run: %w", err)
	}

	docker.PrintSize(outputFile)
	fmt.Println("==> Build complete.")
	return nil
}

// buildBtop builds btop from source using cmake.
func buildBtop(binDir string, vers *versions.Versions) error {
	ver := vers.Get("BTOP_VERSION")
	fmt.Println("  btop (building from source)")

	srcDir, err := os.MkdirTemp("", "devlayer-btop-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	// Download source tarball
	url := fmt.Sprintf("https://github.com/aristocratos/btop/archive/refs/tags/v%s.tar.gz", ver)
	if err := download.TarGzFull(url, srcDir, 0); err != nil {
		return fmt.Errorf("download btop source: %w", err)
	}

	// Find extracted directory
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	var srcRoot string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "btop-") {
			srcRoot = filepath.Join(srcDir, e.Name())
			break
		}
	}
	if srcRoot == "" {
		return fmt.Errorf("btop source directory not found")
	}

	buildDir := filepath.Join(srcRoot, "build")

	// cmake configure
	configure := exec.Command("cmake", "-B", buildDir, "-S", srcRoot,
		"-DCMAKE_BUILD_TYPE=Release",
		"-DBTOP_GPU=OFF",
		"-DBTOP_LTO=ON",
	)
	configure.Env = btopBuildEnv(os.Environ(), runtime.GOOS, findXcodeTool("clang"), findXcodeTool("clang++"), findXcodeSDK())
	configure.Stdout = os.Stdout
	configure.Stderr = os.Stderr
	if err := configure.Run(); err != nil {
		return fmt.Errorf("btop cmake configure: %w", err)
	}

	// cmake build
	build := exec.Command("cmake", "--build", buildDir, "--config", "Release")
	build.Env = btopBuildEnv(os.Environ(), runtime.GOOS, findXcodeTool("clang"), findXcodeTool("clang++"), findXcodeSDK())
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("btop cmake build: %w", err)
	}

	// Find and copy binary — cmake may place it in build/ or build/bin/
	btopBin := filepath.Join(buildDir, "btop")
	if _, err := os.Stat(btopBin); err != nil {
		btopBin = filepath.Join(buildDir, "bin", "btop")
	}
	if err := copyFile(btopBin, filepath.Join(binDir, "btop")); err != nil {
		return fmt.Errorf("copy btop: %w", err)
	}

	return nil
}

// buildNvim builds neovim from source using cmake.
// If head is true, builds from the latest HEAD of the main branch;
// otherwise builds the stable tag (or pinned NVIM_VERSION).
func buildNvim(outDir string, vers *versions.Versions, head bool) error {
	branch := "v" + vers.Get("NVIM_VERSION")
	if head {
		branch = "HEAD"
	}
	fmt.Printf("  nvim (building %s from source)\n", branch)

	srcDir, err := os.MkdirTemp("", "devlayer-nvim-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	cloneArgs := []string{"clone", "--depth", "1"}
	if head {
		cloneArgs = append(cloneArgs, "https://github.com/neovim/neovim.git")
	} else {
		cloneArgs = append(cloneArgs, "--branch", branch, "https://github.com/neovim/neovim.git")
	}
	srcRoot := filepath.Join(srcDir, "neovim")
	cloneArgs = append(cloneArgs, srcRoot)

	clone := exec.Command("git", cloneArgs...)
	clone.Stdout = os.Stdout
	clone.Stderr = os.Stderr
	if err := clone.Run(); err != nil {
		return fmt.Errorf("nvim git clone: %w", err)
	}

	installDir := filepath.Join(outDir, "nvim")

	// cmake configure + build + install
	build := exec.Command("make",
		"CMAKE_BUILD_TYPE=Release",
		fmt.Sprintf("CMAKE_INSTALL_PREFIX=%s", installDir),
		fmt.Sprintf("-j%d", runtime.NumCPU()),
	)
	build.Dir = srcRoot
	build.Env = nvimBuildEnv(os.Environ(), runtime.GOOS, findXcodeTool("clang"), findXcodeTool("clang++"), findXcodeSDK())
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("nvim build: %w", err)
	}

	install := exec.Command("make", "install")
	install.Dir = srcRoot
	install.Env = nvimBuildEnv(os.Environ(), runtime.GOOS, findXcodeTool("clang"), findXcodeTool("clang++"), findXcodeSDK())
	install.Stdout = os.Stdout
	install.Stderr = os.Stderr
	if err := install.Run(); err != nil {
		return fmt.Errorf("nvim install: %w", err)
	}

	return nil
}

func nvimBuildEnv(base []string, goos, clang, clangxx, sdk string) []string {
	return darwinXcodeEnv(base, goos, clang, clangxx, sdk)
}

func btopBuildEnv(base []string, goos, clang, clangxx, sdk string) []string {
	return darwinXcodeEnv(base, goos, clang, clangxx, sdk)
}

func darwinXcodeEnv(base []string, goos, clang, clangxx, sdk string) []string {
	if goos != "darwin" || clang == "" || clangxx == "" || sdk == "" {
		return base
	}
	env := append([]string{}, base...)
	env = append(env, "CC="+clang)
	env = append(env, "CXX="+clangxx)
	env = append(env, "SDKROOT="+sdk)
	return env
}

func findXcodeTool(name string) string {
	path, err := exec.LookPath("xcrun")
	if err != nil {
		return ""
	}
	out, err := exec.Command(path, "--find", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func findXcodeSDK() string {
	path, err := exec.LookPath("xcrun")
	if err != nil {
		return ""
	}
	out, err := exec.Command(path, "--show-sdk-path").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func touchTree(root string) error {
	now := time.Now()
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(path, now, now)
	})
}

func withEnv(env []string, key, value string) []string {
	out := make([]string, 0, len(env)+1)
	prefix := key + "="
	replaced := false
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			out = append(out, prefix+value)
			replaced = true
			continue
		}
		out = append(out, entry)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}

func htopBuildEnv(base []string) []string {
	const systemPath = "/usr/bin:/bin:/opt/homebrew/bin:/usr/sbin:/sbin"
	return withEnv(base, "PATH", systemPath)
}

func ezaBuildEnv(base []string, goos, cargoPath, clang, clangxx, sdk string) []string {
	const systemPath = "/usr/bin:/bin:/opt/homebrew/bin:/usr/sbin:/sbin"
	if goos != "darwin" {
		return base
	}
	env := darwinXcodeEnv(base, goos, clang, clangxx, sdk)
	// Darwin PATH even when this test/binary runs on Windows.
	cargoDir := path.Dir(filepath.ToSlash(cargoPath))
	if cargoDir == "" || cargoDir == "." {
		return withEnv(env, "PATH", systemPath)
	}
	return withEnv(env, "PATH", cargoDir+":"+systemPath)
}

func buildMakeTool(goos string) string {
	if goos == "darwin" {
		return "/usr/bin/make"
	}
	return "make"
}

// buildHtop builds htop from source using autotools.
func buildHtop(binDir string, vers *versions.Versions) error {
	ver := vers.Get("HTOP_VERSION")
	fmt.Println("  htop (building from source)")

	srcDir, err := os.MkdirTemp("", "devlayer-htop-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	// Clone source at the pinned version
	srcRoot := filepath.Join(srcDir, "htop")
	clone := exec.Command("git", "clone", "--depth", "1", "--branch", ver,
		"https://github.com/htop-dev/htop.git", srcRoot)
	clone.Stdout = os.Stdout
	clone.Stderr = os.Stderr
	if err := clone.Run(); err != nil {
		return fmt.Errorf("htop git clone: %w", err)
	}

	// autogen
	autogen := exec.Command("./autogen.sh")
	autogen.Dir = srcRoot
	autogen.Env = htopBuildEnv(os.Environ())
	autogen.Stdout = os.Stdout
	autogen.Stderr = os.Stderr
	if err := autogen.Run(); err != nil {
		return fmt.Errorf("htop autogen: %w", err)
	}

	if err := touchTree(srcRoot); err != nil {
		return fmt.Errorf("htop touch tree: %w", err)
	}
	time.Sleep(2 * time.Second)

	// configure
	configure := exec.Command("./configure", "CFLAGS=-Os -DNDEBUG")
	configure.Dir = srcRoot
	configure.Env = htopBuildEnv(os.Environ())
	configure.Stdout = os.Stdout
	configure.Stderr = os.Stderr
	if err := configure.Run(); err != nil {
		return fmt.Errorf("htop configure: %w", err)
	}

	// build
	make := exec.Command("make", fmt.Sprintf("-j%d", runtime.NumCPU()))
	make.Dir = srcRoot
	make.Env = htopBuildEnv(os.Environ())
	make.Stdout = os.Stdout
	make.Stderr = os.Stderr
	if err := make.Run(); err != nil {
		return fmt.Errorf("htop build: %w", err)
	}

	// copy binary
	if err := copyFile(filepath.Join(srcRoot, "htop"), filepath.Join(binDir, "htop")); err != nil {
		return fmt.Errorf("copy htop: %w", err)
	}

	return nil
}

func downloadAll(out string, p *platform.Platform, vers *versions.Versions, skip map[string]bool) error {
	if err := binaries.Fetch(out, p, vers, skip); err != nil {
		return err
	}
	return installEzaTheme(out)
}

// buildEza compiles eza from source using cargo.
func buildEza(binDir string, vers *versions.Versions) error {
	ver := vers.Get("EZA_VERSION")
	fmt.Println("  eza (building from source)")

	srcDir, err := os.MkdirTemp("", "devlayer-eza-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	// Clone source at the pinned version
	srcRoot := filepath.Join(srcDir, "eza")
	clone := exec.Command("git", "clone", "--depth", "1", "--branch", "v"+ver,
		"https://github.com/eza-community/eza.git", srcRoot)
	clone.Stdout = os.Stdout
	clone.Stderr = os.Stderr
	if err := clone.Run(); err != nil {
		return fmt.Errorf("eza git clone: %w", err)
	}

	// Build with cargo
	cargoPath, err := exec.LookPath("cargo")
	if err != nil {
		return fmt.Errorf("find cargo: %w", err)
	}
	build := exec.Command(cargoPath, "build", "--release")
	build.Dir = srcRoot
	build.Env = ezaBuildEnv(os.Environ(), runtime.GOOS, cargoPath, findXcodeTool("clang"), findXcodeTool("clang++"), findXcodeSDK())
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("eza cargo build: %w", err)
	}

	// Copy binary
	if err := copyFile(filepath.Join(srcRoot, "target", "release", "eza"), filepath.Join(binDir, "eza")); err != nil {
		return fmt.Errorf("copy eza: %w", err)
	}

	return nil
}

// buildMake compiles GNU make from source.
func buildMake(binDir string, vers *versions.Versions) error {
	ver := vers.Get("MAKE_VERSION")
	fmt.Println("  make (building from source)")

	srcDir, err := os.MkdirTemp("", "devlayer-make-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	url := fmt.Sprintf("https://ftp.gnu.org/gnu/make/make-%s.tar.gz", ver)
	if err := download.TarGzFull(url, srcDir, 0); err != nil {
		return fmt.Errorf("download make source: %w", err)
	}

	// Find extracted directory
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	var srcRoot string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "make-") {
			srcRoot = filepath.Join(srcDir, e.Name())
			break
		}
	}
	if srcRoot == "" {
		return fmt.Errorf("make source directory not found")
	}

	if err := touchTree(srcRoot); err != nil {
		return fmt.Errorf("make touch tree: %w", err)
	}
	time.Sleep(2 * time.Second)

	// configure
	configure := exec.Command("./configure", "CFLAGS=-Os -DNDEBUG")
	configure.Dir = srcRoot
	configure.Env = htopBuildEnv(os.Environ())
	configure.Stdout = os.Stdout
	configure.Stderr = os.Stderr
	if err := configure.Run(); err != nil {
		return fmt.Errorf("make configure: %w", err)
	}

	// build
	build := exec.Command(buildMakeTool(runtime.GOOS), fmt.Sprintf("-j%d", runtime.NumCPU()))
	build.Dir = srcRoot
	build.Env = htopBuildEnv(os.Environ())
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("make build: %w", err)
	}

	// copy binary
	if err := copyFile(filepath.Join(srcRoot, "make"), filepath.Join(binDir, "make")); err != nil {
		return fmt.Errorf("copy make: %w", err)
	}

	return nil
}

// createZigCCWrappers creates cc and c++ wrapper scripts that delegate to zig.
func createZigCCWrappers(binDir string) error {
	ccScript := `#!/bin/sh
PREFIX="$(cd "$(dirname "$0")/.." && pwd)"

# Normalize target triples for zig compatibility:
#   arm64 → aarch64    (zig doesn't recognize Apple's "arm64")
#   strip "apple" vendor (zig doesn't recognize it in --target)
#   macosx → macos     (zig uses "macos")
for arg in "$@"; do
  shift
  case "$arg" in
    --target=*) arg=$(echo "$arg" | sed 's/arm64/aarch64/;s/-apple//;s/macosx/macos/') ;;
  esac
  set -- "$@" "$arg"
done

exec "$PREFIX/zig/zig" cc "$@"
`
	cxxScript := `#!/bin/sh
PREFIX="$(cd "$(dirname "$0")/.." && pwd)"

# Normalize target triples for zig compatibility:
#   arm64 → aarch64    (zig doesn't recognize Apple's "arm64")
#   strip "apple" vendor (zig doesn't recognize it in --target)
#   macosx → macos     (zig uses "macos")
for arg in "$@"; do
  shift
  case "$arg" in
    --target=*) arg=$(echo "$arg" | sed 's/arm64/aarch64/;s/-apple//;s/macosx/macos/') ;;
  esac
  set -- "$@" "$arg"
done

exec "$PREFIX/zig/zig" c++ "$@"
`
	if err := os.WriteFile(filepath.Join(binDir, "cc"), []byte(ccScript), 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(binDir, "c++"), []byte(cxxScript), 0755)
}

// createZigCCWrappersWindows creates cc.cmd and c++.cmd wrappers for Windows.
func createZigCCWrappersWindows(binDir string) error {
	ccScript := "@echo off\r\n\"%~dp0..\\zig\\zig.exe\" cc %*\r\n"
	cxxScript := "@echo off\r\n\"%~dp0..\\zig\\zig.exe\" c++ %*\r\n"
	if err := os.WriteFile(filepath.Join(binDir, "cc.cmd"), []byte(ccScript), 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(binDir, "c++.cmd"), []byte(cxxScript), 0755)
}

// buildDotfiles reads the config and creates a dotfiles tarball.
// Skipped silently if no config exists or sync list is empty.
func buildDotfiles(scriptDir string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if len(cfg.Dotfiles.Sync) == 0 {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	fmt.Println("==> Packaging dotfiles...")

	staging, err := os.MkdirTemp("", "devlayer-dotfiles-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	for _, rel := range cfg.Dotfiles.Sync {
		src := filepath.Join(home, rel)
		dst := filepath.Join(staging, rel)

		_, err := os.Stat(src)
		if os.IsNotExist(err) {
			fmt.Printf("  skipped (not found): %s\n", rel)
			continue
		}
		if err != nil {
			return err
		}

		// Resolve symlinks — we want actual content in the tarball
		realSrc, err := filepath.EvalSymlinks(src)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", rel, err)
		}
		realInfo, err := os.Stat(realSrc)
		if err != nil {
			return err
		}

		if realInfo.IsDir() {
			if err := os.MkdirAll(dst, 0755); err != nil {
				return err
			}
			if err := copyDirSimple(realSrc, dst); err != nil {
				return fmt.Errorf("copy %s: %w", rel, err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return err
			}
			if err := copyFile(realSrc, dst); err != nil {
				return fmt.Errorf("copy %s: %w", rel, err)
			}
		}
		fmt.Printf("  %s\n", rel)
	}

	outputFile := filepath.Join(scriptDir, "devlayer-dotfiles.tar.gz")
	if err := archive.CreateTarGz(outputFile, staging); err != nil {
		return err
	}

	docker.PrintSize(outputFile)
	return nil
}

// buildNvimPlugins packages nvim plugins, treesitter parsers, and Mason LSP
// servers into a single tarball for deployment.
// Skipped silently if nvim-pack-lock.json doesn't exist.
func buildNvimPlugins(scriptDir string) error {
	lockfile := nvimplugins.LockfilePath()
	if _, err := os.Stat(lockfile); os.IsNotExist(err) {
		return nil
	}

	fmt.Println("==> Packaging nvim plugins...")

	staging, err := os.MkdirTemp("", "devlayer-nvim-plugins-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	home, _ := os.UserHomeDir()
	nvimData := filepath.Join(home, ".local", "share", "nvim")

	// 1. Plugins from lockfile
	pluginDir := filepath.Join(staging, "site", "pack", "core", "opt")
	if err := nvimplugins.SyncPlugins(lockfile, nvimplugins.LocalPluginDir(), pluginDir); err != nil {
		return fmt.Errorf("sync nvim plugins: %w", err)
	}
	entries, _ := os.ReadDir(pluginDir)
	fmt.Printf("  %d plugins\n", len(entries))

	// 2. Treesitter parsers (.so files) and queries
	parserSrc := filepath.Join(nvimData, "site", "parser")
	if _, err := os.Stat(parserSrc); err == nil {
		parserDst := filepath.Join(staging, "site", "parser")
		if err := copyDirSimple(parserSrc, parserDst); err != nil {
			return fmt.Errorf("copy treesitter parsers: %w", err)
		}
		parsers, _ := os.ReadDir(parserDst)
		fmt.Printf("  %d treesitter parsers\n", len(parsers))
	}
	// Treesitter queries are symlinks to plugin dirs — resolve and copy content
	querySrc := filepath.Join(nvimData, "site", "queries")
	if entries, err := os.ReadDir(querySrc); err == nil {
		queryDst := filepath.Join(staging, "site", "queries")
		if err := os.MkdirAll(queryDst, 0755); err != nil {
			return fmt.Errorf("mkdir queries: %w", err)
		}
		for _, e := range entries {
			src := filepath.Join(querySrc, e.Name())
			realSrc, err := filepath.EvalSymlinks(src)
			if err != nil {
				continue
			}
			if err := copyDirSimple(realSrc, filepath.Join(queryDst, e.Name())); err != nil {
				return fmt.Errorf("copy query %s: %w", e.Name(), err)
			}
		}
	}

	// 3. Mason LSP servers
	masonSrc := filepath.Join(nvimData, "mason")
	if _, err := os.Stat(masonSrc); err == nil {
		masonDst := filepath.Join(staging, "mason")
		if err := copyDirSimple(masonSrc, masonDst); err != nil {
			return fmt.Errorf("copy mason packages: %w", err)
		}
		pkgs, _ := os.ReadDir(filepath.Join(masonDst, "packages"))
		fmt.Printf("  %d mason packages\n", len(pkgs))
	}

	outputFile := filepath.Join(scriptDir, "devlayer-nvim-plugins.tar.gz")
	if err := archive.CreateTarGz(outputFile, staging); err != nil {
		return err
	}

	docker.PrintSize(outputFile)
	return nil
}

// copyDirSimple recursively copies a directory, skipping .git directories.
func copyDirSimple(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		if d.Type()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return nil
			}
			return os.Symlink(linkTarget, target)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func generateChecksums(dir string) error {
	var files []string
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if info.Mode()&0111 != 0 {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("walk %s: %w", dir, err)
	}
	sort.Strings(files)

	f, err := os.Create(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		return err
	}
	defer f.Close()

	for _, path := range files {
		hash, err := sha256File(path)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(dir, path)
		fmt.Fprintf(f, "%s  %s\n", hash, rel)
	}
	return nil
}

func sha256File(path string) (string, error) {
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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode()|0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// FindScriptDir returns the directory containing the devlayer source files.
// It checks for the repo checkout first (Dockerfile exists), then falls back
// to a temp dir with embedded files.
func FindScriptDir(embeddedDockerfile, embeddedVersionsEnv string) (string, bool, error) {
	if d := os.Getenv("BUILD_WORKING_DIRECTORY"); d != "" {
		return d, false, nil
	}

	// Check if we're in a repo checkout
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err == nil {
			return dir, false, nil
		}
	}

	// Check current working directory
	cwd, err := os.Getwd()
	if err == nil {
		if _, err := os.Stat(filepath.Join(cwd, "Dockerfile")); err == nil {
			return cwd, false, nil
		}
	}

	// Fall back to temp dir with embedded files
	tmp, err := os.MkdirTemp("", "devlayer-context-*")
	if err != nil {
		return "", false, err
	}

	if err := os.WriteFile(filepath.Join(tmp, "Dockerfile"), []byte(embeddedDockerfile), 0644); err != nil {
		return "", true, err
	}
	if err := os.WriteFile(filepath.Join(tmp, "versions.env"), []byte(embeddedVersionsEnv), 0644); err != nil {
		return "", true, err
	}
	return tmp, true, nil
}

// wrapper defines a tool that needs a wrapper script to set env vars before
// exec'ing the real binary.
type wrapper struct {
	name     string            // binary name in bin/ (e.g., "git")
	realPath string            // relative path from PREFIX to real binary
	envVars  map[string]string // env var name -> value template ($PREFIX or %PREFIX%)
}

// createUnixWrappers writes POSIX shell wrapper scripts into binDir.
// Each script resolves its own location, sets environment variables, and
// exec's the real binary.
func createUnixWrappers(binDir string, wrappers []wrapper) error {
	for _, w := range wrappers {
		var b strings.Builder
		b.WriteString("#!/bin/sh\n")
		b.WriteString(`PREFIX="$(cd "$(dirname "$0")/.." && pwd)"` + "\n")
		for k, v := range w.envVars {
			expanded := strings.ReplaceAll(v, "$PREFIX", `"$PREFIX"`)
			// For FPATH, append existing value
			if k == "FPATH" {
				fmt.Fprintf(&b, "export %s=%s${%s:+:$%s}\n", k, expanded, k, k)
			} else {
				fmt.Fprintf(&b, "export %s=%s\n", k, expanded)
			}
		}
		fmt.Fprintf(&b, "exec \"$PREFIX/%s\" \"$@\"\n", w.realPath)

		path := filepath.Join(binDir, w.name)
		if err := os.WriteFile(path, []byte(b.String()), 0755); err != nil {
			return fmt.Errorf("wrapper %s: %w", w.name, err)
		}
	}
	return nil
}

// createWindowsWrappers writes .cmd wrapper scripts into binDir.
func createWindowsWrappers(binDir string, wrappers []wrapper) error {
	for _, w := range wrappers {
		var b strings.Builder
		b.WriteString("@echo off\r\n")
		for k, v := range w.envVars {
			expanded := strings.ReplaceAll(v, `%PREFIX%`, `%~dp0..`)
			fmt.Fprintf(&b, "set \"%s=%s\"\r\n", k, expanded)
		}
		realPath := strings.ReplaceAll(w.realPath, `%PREFIX%`, `%~dp0..`)
		fmt.Fprintf(&b, "\"%%~dp0..\\%s\" %%*\r\n", realPath)

		path := filepath.Join(binDir, w.name+".cmd")
		if err := os.WriteFile(path, []byte(b.String()), 0755); err != nil {
			return fmt.Errorf("wrapper %s: %w", w.name, err)
		}
	}
	return nil
}

// VersionSummary returns a multi-line string of tool versions for display.
func VersionSummary(vers *versions.Versions) string {
	keys := []struct{ label, key string }{
		{"fzf", "FZF_VERSION"}, {"fd", "FD_VERSION"}, {"bat", "BAT_VERSION"},
		{"eza", "EZA_VERSION"}, {"rg", "RG_VERSION"}, {"delta", "DELTA_VERSION"},
		{"lazygit", "LAZYGIT_VERSION"}, {"gh", "GH_VERSION"}, {"jq", "JQ_VERSION"},
		{"direnv", "DIRENV_VERSION"},
		{"nvim", "NVIM_VERSION"}, {"go", "GO_VERSION"}, {"git", "GIT_VERSION"},
		{"git-win", "GIT_WINDOWS_VERSION"}, {"zsh", "ZSH_VERSION"},
		{"htop", "HTOP_VERSION"}, {"btop", "BTOP_VERSION"}, {"dust", "DUST_VERSION"},
		{"age", "AGE_VERSION"}, {"zig", "ZIG_VERSION"}, {"make", "MAKE_VERSION"},
		{"ncurses", "NCURSES_VERSION"}, {"batman", "BAT_EXTRAS_VERSION"},
	}
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "  %-10s %s\n", k.label, vers.Get(k.key))
	}
	return b.String()
}
