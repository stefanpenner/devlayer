package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/stefanpenner/devlayer/internal/archive"
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

// Build builds a devlayer bundle for the given OS and architecture.
func Build(args []string, vers *versions.Versions, scriptDir string) error {
	opts := buildOptions{
		os:   "linux",
		arch: runtime.GOARCH,
	}
	// Normalize Go arch to uname style
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
			return fmt.Errorf("unknown option: %s", args[i])
		}
	}

	switch opts.os {
	case "darwin":
		if err := buildDarwin(opts, vers, scriptDir); err != nil {
			return err
		}
	case "windows":
		if err := buildWindows(opts.arch, vers, scriptDir); err != nil {
			return err
		}
	default:
		if err := buildLinux(opts.arch, scriptDir); err != nil {
			return err
		}
	}

	// Build dotfiles and nvim plugins (platform-independent)
	if err := buildDotfiles(scriptDir); err != nil {
		return err
	}
	if err := buildNvimPlugins(scriptDir); err != nil {
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

func buildLinux(arch, scriptDir string) error {
	fmt.Printf("==> Building devlayer for linux/%s (Docker)...\n", arch)

	p, err := platform.New("linux", arch)
	if err != nil {
		return err
	}

	if err := docker.Build(p.DockerPlatform, "devlayer", scriptDir); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}

	outputFile := filepath.Join(scriptDir, fmt.Sprintf("devlayer-linux-%s.tar.gz", arch))
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
	configure.Stdout = os.Stdout
	configure.Stderr = os.Stderr
	if err := configure.Run(); err != nil {
		return fmt.Errorf("btop cmake configure: %w", err)
	}

	// cmake build
	build := exec.Command("cmake", "--build", buildDir, "--config", "Release")
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
	label := "stable"
	if head {
		label = "HEAD"
	}
	fmt.Printf("  nvim (building %s from source)\n", label)

	srcDir, err := os.MkdirTemp("", "devlayer-nvim-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	// Clone source
	cloneArgs := []string{"clone", "--depth", "1"}
	if head {
		cloneArgs = append(cloneArgs, "https://github.com/neovim/neovim.git")
	} else {
		cloneArgs = append(cloneArgs, "--branch", "stable", "https://github.com/neovim/neovim.git")
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

	var current string
	for _, entry := range base {
		if strings.HasPrefix(entry, "PATH=") {
			current = strings.TrimPrefix(entry, "PATH=")
			break
		}
	}
	if current != "" {
		return withEnv(base, "PATH", systemPath+":"+current)
	}
	return append([]string{}, "PATH="+systemPath)
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
	binDir := filepath.Join(out, "bin")
	exe := p.ExeSuffix

	// Rust tools
	rustTools := []struct {
		name, repo, prefix string
		useTargetFor       bool
	}{
		{"fd", "sharkdp/fd", "fd-v%s-%s", false},
		{"bat", "sharkdp/bat", "bat-v%s-%s", false},
		{"rg", "BurntSushi/ripgrep", "ripgrep-%s-%s", true},
		{"delta", "dandavison/delta", "delta-%s-%s", true},
		{"dust", "bootandy/dust", "dust-v%s-%s", true},
	}

	versionKeys := map[string]string{
		"fd": "FD_VERSION", "bat": "BAT_VERSION",
		"rg": "RG_VERSION", "delta": "DELTA_VERSION", "dust": "DUST_VERSION",
	}

	archiveExt := p.RustArchiveExt()

	for _, t := range rustTools {
		if p.SkipTool(t.name) {
			continue
		}
		ver := vers.Get(versionKeys[t.name])
		target := p.RustTarget
		if t.useTargetFor {
			target = p.RustTargetFor(t.name)
		}
		archiveName := fmt.Sprintf(t.prefix, ver, target)
		vPrefix := ver
		if t.name == "fd" || t.name == "bat" || t.name == "dust" {
			vPrefix = "v" + ver
		}
		url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s.%s", t.repo, vPrefix, archiveName, archiveExt)
		if t.name == "rg" || t.name == "delta" {
			url = fmt.Sprintf("https://github.com/%s/releases/download/%s/%s.%s", t.repo, ver, archiveName, archiveExt)
		}
		binaryName := t.name + exe
		if archiveExt == "zip" {
			if err := download.ZipBinary(url, binDir, binaryName); err != nil {
				return fmt.Errorf("download %s: %w", t.name, err)
			}
		} else {
			if err := download.TarGzBinary(url, binDir, binaryName); err != nil {
				return fmt.Errorf("download %s: %w", t.name, err)
			}
		}
	}

	// eza (ls replacement) — built from source on macOS/Linux, download on Windows
	if !skip["eza"] && p.IsWindows() {
		ezaVer := vers.Get("EZA_VERSION")
		ezaTarget := p.RustTargetFor("eza")
		ezaURL := fmt.Sprintf("https://github.com/eza-community/eza/releases/download/v%s/eza.exe_%s.zip",
			ezaVer, ezaTarget)
		if err := download.ZipBinary(ezaURL, binDir, "eza.exe"); err != nil {
			return fmt.Errorf("download eza: %w", err)
		}
	}

	// eza Carbonfox theme
	if err := installEzaTheme(out); err != nil {
		return fmt.Errorf("eza theme: %w", err)
	}

	// Go tools
	fzfVer := vers.Get("FZF_VERSION")
	fzfExt := p.FzfArchiveExt()
	fzfURL := fmt.Sprintf("https://github.com/junegunn/fzf/releases/download/v%s/fzf-%s-%s_%s.%s",
		fzfVer, fzfVer, p.OS, p.GoArch, fzfExt)
	fzfBinary := "fzf" + exe
	if fzfExt == "zip" {
		if err := download.ZipBinary(fzfURL, binDir, fzfBinary); err != nil {
			return fmt.Errorf("download fzf: %w", err)
		}
	} else {
		if err := download.TarGzBinary(fzfURL, binDir, fzfBinary); err != nil {
			return fmt.Errorf("download fzf: %w", err)
		}
	}

	lazygitVer := vers.Get("LAZYGIT_VERSION")
	lazygitExt := p.LazygitArchiveExt()
	lazygitURL := fmt.Sprintf("https://github.com/jesseduffield/lazygit/releases/download/v%s/lazygit_%s_%s_%s.%s",
		lazygitVer, lazygitVer, p.LazygitOS, p.ArchGeneric, lazygitExt)
	lazygitBinary := "lazygit" + exe
	if lazygitExt == "zip" {
		if err := download.ZipBinary(lazygitURL, binDir, lazygitBinary); err != nil {
			return fmt.Errorf("download lazygit: %w", err)
		}
	} else {
		if err := download.TarGzBinary(lazygitURL, binDir, lazygitBinary); err != nil {
			return fmt.Errorf("download lazygit: %w", err)
		}
	}

	// age (encryption tool — two binaries: age, age-keygen)
	ageVer := vers.Get("AGE_VERSION")
	ageExt := "tar.gz"
	if p.IsWindows() {
		ageExt = "zip"
	}
	ageURL := fmt.Sprintf("https://github.com/FiloSottile/age/releases/download/v%s/age-v%s-%s-%s.%s",
		ageVer, ageVer, p.OS, p.GoArch, ageExt)
	for _, ageBin := range []string{"age", "age-keygen"} {
		binaryName := ageBin + exe
		if ageExt == "zip" {
			if err := download.ZipBinary(ageURL, binDir, binaryName); err != nil {
				return fmt.Errorf("download %s: %w", ageBin, err)
			}
		} else {
			if err := download.TarGzBinary(ageURL, binDir, binaryName); err != nil {
				return fmt.Errorf("download %s: %w", ageBin, err)
			}
		}
	}

	// Single-binary tools
	direnvVer := vers.Get("DIRENV_VERSION")
	direnvURL := fmt.Sprintf("https://github.com/direnv/direnv/releases/download/v%s/direnv.%s-%s",
		direnvVer, p.OS, p.GoArch)
	direnvDest := "direnv" + exe
	if err := download.File(direnvURL, filepath.Join(binDir, direnvDest)); err != nil {
		return fmt.Errorf("download direnv: %w", err)
	}

	jqVer := vers.Get("JQ_VERSION")
	jqBinary := "jq" + exe
	var jqURL string
	if p.IsWindows() {
		jqURL = fmt.Sprintf("https://github.com/jqlang/jq/releases/download/jq-%s/jq-windows-%s.exe",
			jqVer, p.GoArch)
	} else {
		jqURL = fmt.Sprintf("https://github.com/jqlang/jq/releases/download/jq-%s/jq-%s-%s",
			jqVer, p.JqOS, p.GoArch)
	}
	if err := download.File(jqURL, filepath.Join(binDir, jqBinary)); err != nil {
		return fmt.Errorf("download jq: %w", err)
	}

	// bat-extras (batman is a shell script — skip on Windows)
	if !p.SkipTool("batman") {
		batExtrasVer := vers.Get("BAT_EXTRAS_VERSION")
		batExtrasURL := fmt.Sprintf("https://github.com/eth-p/bat-extras/releases/download/v%s/bat-extras-%s.zip",
			batExtrasVer, batExtrasVer)
		if err := download.ZipFiles(batExtrasURL, map[string]string{
			fmt.Sprintf("bat-extras-%s/bin/batman", batExtrasVer): filepath.Join(binDir, "batman"),
		}); err != nil {
			// Try alternate zip layout
			if err2 := download.ZipFiles(batExtrasURL, map[string]string{
				"bin/batman": filepath.Join(binDir, "batman"),
			}); err2 != nil {
				return fmt.Errorf("download batman: %w", err)
			}
		}
		fmt.Println("  batman")
	}

	// Neovim (skip if building from source)
	if !skip["nvim"] {
		nvimVer := vers.Get("NVIM_VERSION")
		nvimArchiveName := p.NvimArchiveName(nvimVer)
		nvimExt := p.NvimArchiveExt()
		nvimURL := fmt.Sprintf("https://github.com/neovim/neovim/releases/download/v%s/%s.%s",
			nvimVer, nvimArchiveName, nvimExt)
		fmt.Println("  nvim")
		nvimDir := filepath.Join(out, "nvim")
		if nvimExt == "zip" {
			if err := download.ZipFull(nvimURL, nvimDir, 1); err != nil {
				return fmt.Errorf("download nvim: %w", err)
			}
		} else {
			if err := download.TarGzFull(nvimURL, nvimDir, 1); err != nil {
				return fmt.Errorf("download nvim: %w", err)
			}
		}
	}

	// Go SDK
	goVer := vers.Get("GO_VERSION")
	goExt := p.GoArchiveExt()
	goURL := fmt.Sprintf("https://go.dev/dl/go%s.%s-%s.%s", goVer, p.OS, p.GoArch, goExt)
	fmt.Println("  go")
	if goExt == "zip" {
		if err := download.ZipFull(goURL, out, 0); err != nil {
			return fmt.Errorf("download go: %w", err)
		}
	} else {
		if err := download.TarGzFull(goURL, out, 0); err != nil {
			return fmt.Errorf("download go: %w", err)
		}
	}

	// Zig (C/C++ compiler)
	zigVer := vers.Get("ZIG_VERSION")
	zigArch := p.RustArch // zig uses x86_64/aarch64
	zigExt := "tar.xz"
	if p.IsWindows() {
		zigExt = "zip"
	}
	zigURL := fmt.Sprintf("https://ziglang.org/download/%s/zig-%s-%s-%s.%s",
		zigVer, zigArch, p.ZigOS, zigVer, zigExt)
	fmt.Println("  zig")
	zigDir := filepath.Join(out, "zig")
	if zigExt == "zip" {
		if err := download.ZipFull(zigURL, zigDir, 1); err != nil {
			return fmt.Errorf("download zig: %w", err)
		}
	} else {
		if err := download.TarXzFull(zigURL, zigDir, 1); err != nil {
			return fmt.Errorf("download zig: %w", err)
		}
	}

	// Git for Windows (MinGit portable)
	if p.IsWindows() {
		gitWinVer := vers.Get("GIT_WINDOWS_VERSION")
		gitArch := "64-bit"
		if p.GoArch == "arm64" {
			gitArch = "arm64"
		}
		// Version 2.53.0.2 → tag v2.53.0.windows.2
		parts := strings.Split(gitWinVer, ".")
		winPatch := parts[len(parts)-1]
		gitBase := strings.Join(parts[:len(parts)-1], ".")
		gitURL := fmt.Sprintf("https://github.com/git-for-windows/git/releases/download/v%s.windows.%s/MinGit-%s-%s.zip",
			gitBase, winPatch, gitWinVer, gitArch)
		fmt.Println("  git (MinGit)")
		gitDir := filepath.Join(out, "git")
		if err := download.ZipFull(gitURL, gitDir, 0); err != nil {
			return fmt.Errorf("download git: %w", err)
		}
	}

	// fzf shell integration (not useful on Windows)
	if !p.IsWindows() {
		fmt.Println("  fzf shell integration")
		fzfShellDir := filepath.Join(out, "share", "fzf")
		if err := os.MkdirAll(fzfShellDir, 0755); err != nil {
			return err
		}
		fzfSrcURL := fmt.Sprintf("https://github.com/junegunn/fzf/archive/refs/tags/v%s.tar.gz", fzfVer)
		fzfTmp, err := os.MkdirTemp("", "devlayer-fzf-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(fzfTmp)
		if err := download.TarGzFull(fzfSrcURL, fzfTmp, 0); err != nil {
			return fmt.Errorf("download fzf shell: %w", err)
		}
		// Find and copy shell files
		entries, err := os.ReadDir(fzfTmp)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				shellDir := filepath.Join(fzfTmp, e.Name(), "shell")
				if _, err := os.Stat(shellDir); err == nil {
					for _, name := range []string{"key-bindings.zsh", "completion.zsh"} {
						src := filepath.Join(shellDir, name)
						if _, err := os.Stat(src); err == nil {
							copyFile(src, filepath.Join(fzfShellDir, name))
						}
					}
					break
				}
			}
		}
	}

	// Zsh plugins (not useful on Windows)
	if !p.IsWindows() {
		shareDir := filepath.Join(out, "share")
		plugins := []struct {
			name, repo, versionKey string
		}{
			{"zsh-autosuggestions", "zsh-users/zsh-autosuggestions", "ZSH_AUTOSUGGESTIONS_VERSION"},
			{"zsh-fast-syntax-highlighting", "zdharma-continuum/fast-syntax-highlighting", "FAST_SYNTAX_HIGHLIGHTING_VERSION"},
			{"zsh-history-substring-search", "zsh-users/zsh-history-substring-search", "ZSH_HISTORY_SUBSTRING_SEARCH_VERSION"},
			{"powerlevel10k", "romkatv/powerlevel10k", "POWERLEVEL10K_VERSION"},
		}
		for _, plug := range plugins {
			ver := vers.Get(plug.versionKey)
			url := fmt.Sprintf("https://github.com/%s/archive/refs/tags/%s.tar.gz", plug.repo, ver)
			fmt.Printf("  %s\n", plug.name)
			destDir := filepath.Join(shareDir, plug.name)
			if err := download.TarGzToDir(url, destDir); err != nil {
				return fmt.Errorf("download %s: %w", plug.name, err)
			}
		}
	}

	// chmod +x all binaries
	entries2, _ := os.ReadDir(binDir)
	for _, e := range entries2 {
		os.Chmod(filepath.Join(binDir, e.Name()), 0755)
	}

	fmt.Printf("==> Done: %d binaries + nvim + go", len(entries2))
	if !p.IsWindows() {
		fmt.Print(" + plugins")
	}
	fmt.Println()
	return nil
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
	build := exec.Command("cargo", "build", "--release")
	build.Dir = srcRoot
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

	// configure
	configure := exec.Command("./configure", "CFLAGS=-Os -DNDEBUG")
	configure.Dir = srcRoot
	configure.Stdout = os.Stdout
	configure.Stderr = os.Stderr
	if err := configure.Run(); err != nil {
		return fmt.Errorf("make configure: %w", err)
	}

	// build
	build := exec.Command("make", fmt.Sprintf("-j%d", runtime.NumCPU()))
	build.Dir = srcRoot
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
func FindScriptDir(embeddedDockerfile, embeddedVersionsEnv, embeddedDownloadScript string) (string, bool, error) {
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
	scriptsDir := filepath.Join(tmp, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return "", true, err
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "download-binaries.sh"), []byte(embeddedDownloadScript), 0755); err != nil {
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
		{"lazygit", "LAZYGIT_VERSION"}, {"jq", "JQ_VERSION"}, {"direnv", "DIRENV_VERSION"},
		{"nvim", "NVIM_VERSION"}, {"go", "GO_VERSION"}, {"git", "GIT_VERSION"},
		{"htop", "HTOP_VERSION"}, {"btop", "BTOP_VERSION"}, {"dust", "DUST_VERSION"},
		{"age", "AGE_VERSION"}, {"zig", "ZIG_VERSION"}, {"make", "MAKE_VERSION"},
	}
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "  %-10s %s\n", k.label, vers.Get(k.key))
	}
	return b.String()
}
