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
	"github.com/stefanpenner/devlayer/internal/assemble"
	"github.com/stefanpenner/devlayer/internal/binaries"
	"github.com/stefanpenner/devlayer/internal/config"
	"github.com/stefanpenner/devlayer/internal/deadstrip"
	"github.com/stefanpenner/devlayer/internal/download"
	"github.com/stefanpenner/devlayer/internal/loadcheck"
	"github.com/stefanpenner/devlayer/internal/nvimplugins"
	"github.com/stefanpenner/devlayer/internal/platform"
	"github.com/stefanpenner/devlayer/internal/sums"
	"github.com/stefanpenner/devlayer/internal/terminfo"
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
	// Zig names the arch aarch64. Apple and Go name it arm64.
	switch opts.arch {
	case "amd64":
		opts.arch = "x86_64"
	case "arm64":
		opts.arch = "aarch64"
	}
	return opts, nil
}

// Build builds the sysroot for one OS and one architecture.
// Artifacts are written to outDir. scriptDir is the source checkout.
func Build(args []string, vers *versions.Versions, scriptDir, outDir string) error {
	opts, err := parseBuildOpts(args, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	if err := ensureBininfo(scriptDir); err != nil {
		return err
	}

	switch opts.os {
	case "darwin":
		if err := buildDarwin(opts, vers, scriptDir, outDir); err != nil {
			return err
		}
	case "windows":
		if err := buildWindows(opts.arch, vers, scriptDir, outDir); err != nil {
			return err
		}
	default:
		if err := buildLinux(opts.arch, scriptDir, outDir); err != nil {
			return err
		}
	}

	if err := buildDotfiles(outDir); err != nil {
		return err
	}
	if err := buildPrivate(outDir); err != nil {
		return err
	}
	if err := buildNvimPlugins(outDir); err != nil {
		return err
	}
	return stampNvimSysroots(scriptDir, outDir)
}

func buildDarwin(opts buildOptions, vers *versions.Versions, scriptDir, outDir string) error {
	cc, cleanup, err := pinnedDarwinCompiler(scriptDir, opts.arch)
	if err != nil {
		return err
	}
	defer cleanup()

	p, root, binDir, err := stage("darwin", opts.arch)
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)

	if err := fetchPrebuilts(root, p, vers, map[string]bool{"nvim": true}); err != nil {
		return err
	}
	if err := compileDarwinTools(root, binDir, scriptDir, vers, opts.nvimHead, cc, p.GoArch); err != nil {
		return err
	}
	if err := writeDarwinWrappers(binDir); err != nil {
		return err
	}
	if err := installBundleSelf(binDir, "darwin"); err != nil {
		return err
	}
	output := filepath.Join(outDir, fmt.Sprintf("devlayer-darwin-%s.tar.gz", p.ArchGeneric))
	return finishBundle(root, output, p.BundleExt)
}

func buildWindows(arch string, vers *versions.Versions, scriptDir, outDir string) error {
	p, root, binDir, err := stage("windows", arch)
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)

	if err := fetchPrebuilts(root, p, vers, nil); err != nil {
		return err
	}
	if err := buildGlow(root, binDir, scriptDir, vers, p.OS, p.GoArch); err != nil {
		return err
	}
	if err := writeWindowsWrappers(binDir); err != nil {
		return err
	}
	if err := installBundleSelf(binDir, "windows"); err != nil {
		return err
	}
	output := filepath.Join(outDir, fmt.Sprintf("devlayer-windows-%s.zip", p.ArchGeneric))
	return finishBundle(root, output, p.BundleExt)
}

func stage(goos, arch string) (p *platform.Platform, root, binDir string, err error) {
	p, err = platform.New(goos, arch)
	if err != nil {
		return nil, "", "", err
	}

	fmt.Printf("==> Building devlayer for %s/%s...\n", goos, p.ArchGeneric)
	root, err = os.MkdirTemp("", "devlayer-build-*")
	if err != nil {
		return nil, "", "", err
	}
	binDir = filepath.Join(root, "bin")
	if err = os.MkdirAll(binDir, 0755); err != nil {
		os.RemoveAll(root)
		return nil, "", "", err
	}
	return p, root, binDir, nil
}

func fetchPrebuilts(root string, p *platform.Platform, vers *versions.Versions, skip map[string]bool) error {
	fmt.Println("==> Downloading binaries (" + p.OS + "/" + p.ArchGeneric + ")")
	return downloadAll(root, p, vers, skip)
}

func compileDarwinTools(root, binDir, scriptDir string, vers *versions.Versions, nvimHead bool, cc []string, goarch string) error {
	fmt.Println("==> Building tools from source...")
	if err := buildDarwinStaticLibs(vers, cc, scriptDir); err != nil {
		return err
	}
	if err := buildNvim(root, scriptDir, vers, nvimHead, cc); err != nil {
		return err
	}
	if err := buildHtop(binDir, vers, cc); err != nil {
		return err
	}
	if err := buildBtop(binDir, vers, cc); err != nil {
		return err
	}

	libexecDir := filepath.Join(root, "libexec")
	if err := os.MkdirAll(libexecDir, 0755); err != nil {
		return fmt.Errorf("mkdir libexec: %w", err)
	}
	if err := buildEza(libexecDir, vers, cc); err != nil {
		return err
	}
	if err := buildGlow(root, binDir, scriptDir, vers, "darwin", goarch); err != nil {
		return err
	}
	if err := installHostStub(binDir, scriptDir); err != nil {
		return err
	}
	return buildMake(binDir, vers, cc)
}

// installHostStub copies the Darwin host stub.
// The file name selects the app. The archive is not a build input.
// The names match the hoststub catalog, plus each alias.
func installHostStub(binDir, scriptDir string) error {
	fmt.Println("  host stub")
	bin, err := bazelOutput(scriptDir, "//hoststub:hoststub_darwin_arm64")
	if err != nil {
		return err
	}
	for _, name := range []string{"terminal-browser", "grok", "agent", "termium", "simserve"} {
		out := filepath.Join(binDir, name)
		if err := copyFile(bin, out); err != nil {
			return fmt.Errorf("copy host stub: %w", err)
		}
		if err := os.Chmod(out, 0o755); err != nil {
			return err
		}
		if err := sealDarwin(out); err != nil {
			return err
		}
	}
	return nil
}

func writeDarwinWrappers(binDir string) error {
	script := `#!/bin/sh
PREFIX="$(cd "$(dirname "$0")/.." && pwd)"
export EZA_CONFIG_DIR="$PREFIX/share/eza"
exec "$PREFIX/libexec/eza" "$@"
`
	for _, name := range []string{"eza", "ls"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0755); err != nil {
			return fmt.Errorf("wrapper %s: %w", name, err)
		}
	}

	wrappers := []wrapper{
		{"nvim", "nvim/bin/nvim", map[string]string{"VIMRUNTIME": "$PREFIX/nvim/share/nvim/runtime"}},
		{"go", "go/bin/go", map[string]string{"GOROOT": "$PREFIX/go"}},
		{"gofmt", "go/bin/gofmt", map[string]string{"GOROOT": "$PREFIX/go"}},
		{"zig", "zig/zig", nil},
	}
	if err := createUnixWrappers(binDir, wrappers); err != nil {
		return err
	}
	return createZigCCWrappers(binDir)
}

func writeWindowsWrappers(binDir string) error {
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
	return createZigCCWrappersWindows(binDir)
}

// bundleSelfName is the CLI filename copied from the builder.
// A different target OS gets no copy. The pipeline injects that CLI.
func bundleSelfName(hostOS, targetOS string) (string, bool) {
	if hostOS != targetOS {
		return "", false
	}
	if targetOS == "windows" {
		return "devlayer.exe", true
	}
	return "devlayer", true
}

func installBundleSelf(binDir, targetOS string) error {
	name, ok := bundleSelfName(runtime.GOOS, targetOS)
	if !ok {
		return nil
	}
	return installSelf(binDir, name)
}

func installSelf(binDir, name string) error {
	self, err := os.Executable()
	if err != nil {
		return nil
	}
	fmt.Println("  devlayer")
	if err := copyFile(self, filepath.Join(binDir, name)); err != nil {
		return fmt.Errorf("copy devlayer: %w", err)
	}
	return nil
}

func finishBundle(root, output, ext string) error {
	if err := loadcheck.Tree(root); err != nil {
		return err
	}
	if err := generateChecksums(root); err != nil {
		return err
	}
	var err error
	if ext == "zip" {
		err = archive.CreateZip(output, root)
	} else {
		err = archive.CreateTarGz(output, root)
	}
	if err != nil {
		return err
	}
	printSize(output)
	fmt.Println("==> Build complete.")
	return nil
}

func printSize(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	mb := float64(info.Size()) / 1024 / 1024
	fmt.Printf("  %s (%.0f MB)\n", path, mb)
}

func buildLinux(arch, scriptDir, outDir string) error {
	if err := requireBazelCheckout(scriptDir); err != nil {
		return err
	}
	return bazelLinuxBundle(arch, scriptDir, outDir)
}

func requireBazelCheckout(scriptDir string) error {
	if _, err := exec.LookPath("bazel"); err != nil {
		return fmt.Errorf("build needs bazel on PATH")
	}
	mod := filepath.Join(scriptDir, "MODULE.bazel")
	if _, err := os.Stat(mod); err != nil {
		return fmt.Errorf("build needs %s", mod)
	}
	return nil
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
	printSize(dst)
	fmt.Println("==> Build complete.")
	return nil
}

// pinnedDarwinCompiler builds the CC and CXX wrappers.
// The compiler is the pinned Zig. The SDK is the pinned Apple package.
// codesign stays outside this helper.
func pinnedDarwinCompiler(scriptDir, arch string) (cc []string, cleanup func(), err error) {
	cleanup = func() {}
	if err = requireBazelCheckout(scriptDir); err != nil {
		return nil, cleanup, err
	}
	zig, err := bazelOutput(scriptDir, "@zig_sdk//:zig")
	if err != nil {
		return nil, cleanup, err
	}
	tarPath, err := bazelOutput(scriptDir, "@macos_sdk//:sdk.tar")
	if err != nil {
		return nil, cleanup, err
	}

	work, err := os.MkdirTemp("", "devlayer-sdk-*")
	if err != nil {
		return nil, cleanup, err
	}
	cleanup = func() { os.RemoveAll(work) }
	cmd := exec.Command("tar", "-xf", tarPath, "-C", work)
	if err = cmd.Run(); err != nil {
		return nil, cleanup, fmt.Errorf("extract sdk: %w", err)
	}
	sdk := filepath.Join(work, "MacOSX.sdk")
	info, err := os.Stat(filepath.Join(sdk, "usr", "include"))
	if err != nil || !info.IsDir() {
		return nil, cleanup, fmt.Errorf("pinned sdk has no usr/include")
	}

	staticDir := filepath.Join(work, "static")
	if err = os.MkdirAll(staticDir, 0755); err != nil {
		return nil, cleanup, err
	}
	ccPath := filepath.Join(work, "cc")
	cxxPath := filepath.Join(work, "c++")
	if err = os.WriteFile(ccPath, []byte(darwinCCScript(zig, sdk, arch, "cc", staticDir)), 0755); err != nil {
		return nil, cleanup, err
	}
	if err = os.WriteFile(cxxPath, []byte(darwinCCScript(zig, sdk, arch, "c++", staticDir)), 0755); err != nil {
		return nil, cleanup, err
	}
	return []string{"CC=" + ccPath, "CXX=" + cxxPath, "SDKROOT=" + sdk, "DEVLAYER_STATIC=" + staticDir}, cleanup, nil
}

func darwinCCScript(zig, sdk, arch, tool, staticLib string) string {
	// Zig ignores -bundle and then looks for main.
	// A Mach-O bundle is a dylib with dynamic lookup.
	// -F is the link search. -iframework is the header search.
	// -isystem is the SDK C header dir. Zig does not add it from --sysroot.
	// A .tbd install-name is a host path. Do not search the SDK lib dir.
	// CMake marks the iconv include as -isystem.
	// Clang then reads the SDK iconv.h first. Drop that -isystem.
	fw := sdk + "/System/Library/Frameworks"
	libdir := filepath.Join(sdk, "usr", "lib")
	include := ""
	iconv, wide, narrow, iconvInc := "", "", "", ""
	if staticLib != "" {
		iconv = filepath.Join(staticLib, "iconv", "lib", "libiconv.a")
		wide = filepath.Join(staticLib, "ncurses", "lib", "libncursesw.a")
		narrow = filepath.Join(staticLib, "ncurses", "lib", "libncurses.a")
		iconvInc = filepath.Join(staticLib, "iconv", "include")
		include = fmt.Sprintf("-I %q ", iconvInc)
	}
	return fmt.Sprintf(`#!/bin/sh
# Do not call xcrun. The host SDK stays out.
export DEVELOPER_DIR=/tmp/devlayer-no-xcrun
# A -l flag may appear before its -L flag. Record the directories first.
# Do not keep -L. A search path lets the linker record a stub install-name.
libdir=%q
iconv=%q
wide=%q
narrow=%q
iconv_inc=%q
dirs=""
pendir=0
for arg in "$@"; do
  if [ "$pendir" = 1 ]; then
    pendir=0
    dirs="$dirs
$arg"
    continue
  fi
  case "$arg" in
    -L) pendir=1 ;;
    -L*)
      dirs="$dirs
${arg#-L}"
      ;;
  esac
done
n=$#
i=0
pendir=0
pendlib=0
pendsys=0
while [ "$i" -lt "$n" ]; do
  arg=$1
  shift
  i=$((i + 1))
  if [ "$pendir" = 1 ]; then
    pendir=0
    continue
  fi
  if [ "$pendsys" = 1 ]; then
    pendsys=0
    if [ -n "$iconv_inc" ] && [ "$arg" = "$iconv_inc" ]; then
      continue
    fi
    set -- "$@" -isystem "$arg"
    continue
  fi
  if [ "$pendlib" = 1 ]; then
    pendlib=0
    arg="-l$arg"
  fi
  case "$arg" in
    -bundle) set -- "$@" -dynamiclib -Wl,-undefined,dynamic_lookup ;;
    -Wl,-export_dynamic) set -- "$@" -rdynamic ;;
    -Wl,-l*|-Wl,-L*)
      printf 'refusing %%s\n' "$arg" >&2
      exit 1
      ;;
    --target=*)
      arg=$(printf '%%s\n' "$arg" | sed 's/arm64/aarch64/;s/-apple//;s/macosx/macos/')
      set -- "$@" "$arg"
      ;;
    -L) pendir=1 ;;
    -L*) ;;
    -isystem) pendsys=1 ;;
    -isystem*)
      rest=${arg#-isystem}
      if [ -n "$iconv_inc" ] && [ "$rest" = "$iconv_inc" ]; then
        :
      else
        set -- "$@" "$arg"
      fi
      ;;
    -l) pendlib=1 ;;
    -liconv)
      if [ -f "$iconv" ]; then set -- "$@" "$iconv"
      else printf 'refusing %%s\n' "$arg" >&2; exit 1; fi
      ;;
    -lncurses|-lncursesw)
      if [ -f "$wide" ]; then set -- "$@" "$wide"
      elif [ -f "$narrow" ]; then set -- "$@" "$narrow"
      else printf 'refusing %%s\n' "$arg" >&2; exit 1; fi
      ;;
    -lutil)
      # Zig does not search for libutil unless -L is set. Pass the stub file.
      # libutil is not libSystem, so this does not add a second libSystem load.
      set -- "$@" "$libdir/libutil.tbd"
      ;;
    -lSystem|-lc|-lm|-ldl|-lpthread|-lc++|-lc++abi|-lobjc)
      # Keep the -l flag. These stubs are libSystem or the C++ runtime.
      # A .tbd path is a second load of the same install-name. dyld aborts.
      set -- "$@" "$arg"
      ;;
    -l*)
      name=${arg#-l}
      found=""
      oldifs=$IFS
      IFS='
'
      set -f
      for d in $dirs; do
        [ -n "$d" ] || continue
        rest=${d#/opt/homebrew/}
        if [ "$rest" != "$d" ] || [ "$d" = /opt/homebrew ]; then
          continue
        fi
        rest=${d#/usr/local/}
        if [ "$rest" != "$d" ] || [ "$d" = /usr/local ]; then
          continue
        fi
        if [ -f "$d/lib$name.a" ]; then
          found="$d/lib$name.a"
          break
        fi
      done
      set +f
      IFS=$oldifs
      if [ -n "$found" ]; then set -- "$@" "$found"
      else printf 'refusing %%s\n' "$arg" >&2; exit 1; fi
      ;;
    /usr/lib/libSystem.B.dylib|/usr/lib/libobjc.A.dylib|/usr/lib/libc++.1.dylib|/usr/lib/libc++abi.dylib|/usr/lib/libutil.dylib)
      set -- "$@" "$arg"
      ;;
    /opt/homebrew/*|/usr/local/*)
      case "$arg" in
        *.dylib|*.tbd|*.so)
          printf 'refusing %%s\n' "$arg" >&2
          exit 1
          ;;
      esac
      set -- "$@" "$arg"
      ;;
    /usr/lib/*)
      printf 'refusing %%s\n' "$arg" >&2
      exit 1
      ;;
    *) set -- "$@" "$arg" ;;
  esac
done
if [ "$pendlib" = 1 ]; then
  printf 'refusing -l\n' >&2
  exit 1
fi
# Zig 0.15 instruments C unless the sanitize flag is off.
exec %q %s -target %s-macos --sysroot %q %s-isystem %q -iframework %q -F %q -mmacosx-version-min=11.0 -fno-sanitize=undefined "$@"
`, libdir, iconv, wide, narrow, iconvInc, zig, tool, arch, sdk, include, sdk+"/usr/include", fw, fw)
}

func ensureBininfo(scriptDir string) error {
	if os.Getenv("BININFO_POLICY") == "" {
		policy := filepath.Join(scriptDir, "bininfo", "policy")
		if st, err := os.Stat(policy); err == nil && st.IsDir() {
			if err := os.Setenv("BININFO_POLICY", policy); err != nil {
				return err
			}
		}
	}
	if os.Getenv("BININFO") != "" {
		return nil
	}
	if _, err := os.Stat(filepath.Join(scriptDir, "MODULE.bazel")); err != nil {
		return nil
	}
	bin, err := bazelOutput(scriptDir, "//bininfo")
	if err != nil {
		return err
	}
	return os.Setenv("BININFO", bin)
}

func bazelOutput(scriptDir, label string) (string, error) {
	build := exec.Command("bazel", "build", label)
	build.Dir = scriptDir
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return "", fmt.Errorf("bazel build %s: %w", label, err)
	}

	query := exec.Command("bazel", "cquery", label, "--output=files")
	query.Dir = scriptDir
	out, err := query.Output()
	if err != nil {
		return "", fmt.Errorf("bazel cquery %s: %w", label, err)
	}
	rel := ""
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "INFO:") {
			continue
		}
		rel = line
	}
	if rel == "" {
		return "", fmt.Errorf("bazel cquery %s returned no file", label)
	}
	execRoot, err := bazelInfo(scriptDir, "execution_root")
	if err != nil {
		return "", err
	}
	outputBase, err := bazelInfo(scriptDir, "output_base")
	if err != nil {
		return "", err
	}
	return resolveBazelFile(rel, scriptDir, execRoot, outputBase)
}

// resolveBazelFile finds a cquery path.
// A build output sits under the execution root.
// An external file sits under the output base.
func resolveBazelFile(rel, scriptDir, execRoot, outputBase string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("bazel cquery returned no file")
	}
	if filepath.IsAbs(rel) {
		if _, err := os.Stat(rel); err != nil {
			return "", fmt.Errorf("bazel file missing: %s", rel)
		}
		return rel, nil
	}
	// A later bazel build drops external files from the execution root.
	// The output base copy stays.
	roots := []string{execRoot, outputBase, scriptDir}
	if strings.HasPrefix(rel, "external/") || strings.HasPrefix(rel, "external"+string(filepath.Separator)) {
		roots = []string{outputBase, scriptDir, execRoot}
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		full := filepath.Join(root, rel)
		if _, err := os.Stat(full); err == nil {
			return full, nil
		}
	}
	return "", fmt.Errorf("bazel file missing: %s", rel)
}

func bazelInfo(scriptDir, key string) (string, error) {
	info := exec.Command("bazel", "info", key)
	info.Dir = scriptDir
	out, err := info.Output()
	if err != nil {
		return "", fmt.Errorf("bazel info %s: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func withCompiler(base, cc []string) []string {
	env := append([]string{}, base...)
	env = append(env, cc...)
	env = appendLinkFlag(env, "LDFLAGS", "-Wl,-dead_strip_dylibs")
	if len(cc) == 0 {
		return env
	}
	return blockHostPkgConfig(env)
}

// buildBtop builds btop from source using cmake.
func buildBtop(binDir string, vers *versions.Versions, cc []string) error {
	fmt.Println("  btop (building from source)")

	srcDir, err := os.MkdirTemp("", "devlayer-btop-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	url := binaries.SourceArchiveURLs(vers)["btop"]
	sum, err := sums.Must(url)
	if err != nil {
		return err
	}
	if err := download.TarGzFull(url, srcDir, 0, sum); err != nil {
		return fmt.Errorf("download btop source: %w", err)
	}
	srcRoot, err := tarballRoot(srcDir, "btop-", "btop")
	if err != nil {
		return err
	}

	buildDir := filepath.Join(srcRoot, "build")
	env := withCompiler(os.Environ(), cc)
	if err := run("", env, "cmake", "-B", buildDir, "-S", srcRoot,
		"-DCMAKE_BUILD_TYPE=Release",
		"-DBTOP_GPU=OFF",
		// Zig fails CMake's LTO check.
		"-DBTOP_LTO=OFF",
	); err != nil {
		return fmt.Errorf("btop cmake configure: %w", err)
	}
	if err := run("", env, "cmake", "--build", buildDir, "--config", "Release"); err != nil {
		return fmt.Errorf("btop cmake build: %w", err)
	}

	btopBin := filepath.Join(buildDir, "btop")
	if _, err := os.Stat(btopBin); err != nil {
		btopBin = filepath.Join(buildDir, "bin", "btop")
	}
	if err := copyFile(btopBin, filepath.Join(binDir, "btop")); err != nil {
		return fmt.Errorf("copy btop: %w", err)
	}
	return sealDarwin(filepath.Join(binDir, "btop"))
}

// buildNvim builds neovim from source using cmake.
// If head is true, builds from the latest HEAD of the main branch;
// otherwise builds the stable tag (or pinned NVIM_VERSION).
func buildNvim(outDir, scriptDir string, vers *versions.Versions, head bool, cc []string) error {
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

	var srcRoot string
	if head {
		srcRoot = filepath.Join(srcDir, "neovim")
		if err := run("", nil, "git", "clone", "--depth", "1", "https://github.com/neovim/neovim.git", srcRoot); err != nil {
			return fmt.Errorf("nvim git clone: %w", err)
		}
	} else {
		url := binaries.SourceArchiveURLs(vers)["nvim"]
		sum, err := sums.Must(url)
		if err != nil {
			return err
		}
		if err := download.TarGzFull(url, srcDir, 0, sum); err != nil {
			return fmt.Errorf("download nvim source: %w", err)
		}
		srcRoot, err = tarballRoot(srcDir, "neovim-", "nvim")
		if err != nil {
			return err
		}
	}

	if err := patchNvimTree(srcRoot, scriptDir); err != nil {
		return err
	}
	if err := pinNvimDeps(srcRoot); err != nil {
		return err
	}

	installDir := filepath.Join(outDir, "nvim")
	env := withCompiler(os.Environ(), cc)
	env = withEnv(env, "DEVLAYER_UNIBI_PATCH", filepath.Join(scriptDir, "third_party", "patches", "unibilium_no_host.patch"))
	args := append(nvimMakeArgs(installDir, staticPrefix(cc)), fmt.Sprintf("-j%d", runtime.NumCPU()))
	if err := run(srcRoot, env, "make", args...); err != nil {
		return fmt.Errorf("nvim build: %w", err)
	}
	if err := run(srcRoot, env, "make", "install"); err != nil {
		return fmt.Errorf("nvim install: %w", err)
	}
	if err := deadstrip.Tree(installDir); err != nil {
		return err
	}
	return deadstrip.CheckTree(installDir)
}

func darwinXcodeEnv(base []string, goos, clang, clangxx, sdk string) []string {
	if goos != "darwin" || clang == "" || clangxx == "" || sdk == "" {
		return base
	}
	env := append([]string{}, base...)
	env = append(env, "CC="+clang)
	env = append(env, "CXX="+clangxx)
	env = append(env, "SDKROOT="+sdk)
	return appendLinkFlag(env, "LDFLAGS", "-Wl,-dead_strip_dylibs")
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

func settleTree(root, tool string) error {
	if err := touchTree(root); err != nil {
		return fmt.Errorf("%s touch tree: %w", tool, err)
	}
	time.Sleep(2 * time.Second)
	return nil
}

func tarballRoot(dir, prefix, tool string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("%s source directory not found", tool)
}

func run(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if env != nil {
		cmd.Env = env
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func appendLinkFlag(base []string, key, flag string) []string {
	prefix := key + "="
	for i, entry := range base {
		if !strings.HasPrefix(entry, prefix) {
			continue
		}
		val := strings.TrimPrefix(entry, prefix)
		if strings.Contains(val, flag) {
			return base
		}
		out := append([]string{}, base...)
		if val == "" {
			out[i] = prefix + flag
		} else {
			out[i] = prefix + val + " " + flag
		}
		return out
	}
	return append(base, prefix+flag)
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
	const systemPath = "/usr/bin:/bin:/usr/sbin:/sbin"
	env := withEnv(base, "PATH", systemPath)
	env = blockHostPkgConfig(env)
	if runtime.GOOS == "darwin" {
		env = appendLinkFlag(env, "LDFLAGS", "-Wl,-dead_strip_dylibs")
	}
	return env
}

func ezaBuildEnv(base []string, goos, cargoPath, clang, clangxx, sdk string) []string {
	const systemPath = "/usr/bin:/bin:/usr/sbin:/sbin"
	if goos != "darwin" {
		return base
	}
	env := darwinXcodeEnv(base, goos, clang, clangxx, sdk)
	env = blockHostPkgConfig(env)
	env = withEnv(env, "LIBZ_SYS_STATIC", "1")
	env = appendLinkFlag(env, "RUSTFLAGS", "-C link-arg=-dead_strip_dylibs")
	// rustc links with cc on PATH. CC is only the C compiler.
	env = appendLinkFlag(env, "RUSTFLAGS", "-C linker="+clang)
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
func buildHtop(binDir string, vers *versions.Versions, cc []string) error {
	fmt.Println("  htop (building from source)")

	srcDir, err := os.MkdirTemp("", "devlayer-htop-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	url := binaries.SourceArchiveURLs(vers)["htop"]
	sum, err := sums.Must(url)
	if err != nil {
		return err
	}
	if err := download.TarXzFull(url, srcDir, 0, sum); err != nil {
		return fmt.Errorf("download htop source: %w", err)
	}
	srcRoot, err := tarballRoot(srcDir, "htop-", "htop")
	if err != nil {
		return err
	}

	env := htopBuildEnv(withCompiler(os.Environ(), cc))
	if err := settleTree(srcRoot, "htop"); err != nil {
		return err
	}
	if err := run(srcRoot, env, "./configure", htopConfigureArgs(staticPrefix(cc))...); err != nil {
		return configureErr("htop", srcRoot, err)
	}
	if err := run(srcRoot, env, "make", fmt.Sprintf("-j%d", runtime.NumCPU())); err != nil {
		return fmt.Errorf("htop build: %w", err)
	}
	if err := copyFile(filepath.Join(srcRoot, "htop"), filepath.Join(binDir, "htop")); err != nil {
		return fmt.Errorf("copy htop: %w", err)
	}
	return sealDarwin(filepath.Join(binDir, "htop"))
}

func downloadAll(out string, p *platform.Platform, vers *versions.Versions, skip map[string]bool) error {
	if err := binaries.Fetch(out, p, vers, skip); err != nil {
		return err
	}
	return installEzaTheme(out)
}

// buildGlow compiles glow from the pinned tag archive.
func buildGlow(root, binDir, scriptDir string, vers *versions.Versions, targetOS, goarch string) error {
	fmt.Println("  glow (building from source)")
	goBin, cleanup, err := hostGo(root, vers, targetOS)
	if err != nil {
		return err
	}
	defer cleanup()

	srcDir, err := os.MkdirTemp("", "devlayer-glow-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	url := binaries.SourceArchiveURLs(vers)["glow"]
	sum, err := sums.Must(url)
	if err != nil {
		return err
	}
	if err := download.TarGzFull(url, srcDir, 1, sum); err != nil {
		return fmt.Errorf("download glow source: %w", err)
	}

	env := glowBuildEnv(os.Environ(), goBin, targetOS, goarch, "https://proxy.golang.org,direct", "sum.golang.org", "")
	if err := run(srcDir, env, goBin, "mod", "vendor"); err != nil {
		return fmt.Errorf("glow vendor: %w", err)
	}

	verPath := filepath.Join(srcDir, "version.txt")
	ver := vers.Get("GLOW_VERSION")
	if err := os.WriteFile(verPath, []byte(ver+"\n"), 0o644); err != nil {
		return err
	}
	outName := "glow"
	if targetOS == "windows" {
		outName = "glow.exe"
	}
	out := filepath.Join(binDir, outName)
	script := filepath.Join(scriptDir, "tools", "glow_build.sh")
	if err := run("", nil, script, goBin, srcDir, out, targetOS, goarch, verPath); err != nil {
		return fmt.Errorf("glow build: %w", err)
	}
	if targetOS == "darwin" {
		return sealDarwin(out)
	}
	return nil
}

func hostGo(root string, vers *versions.Versions, targetOS string) (string, func(), error) {
	noop := func() {}
	if runtime.GOOS == targetOS {
		name := "go"
		if runtime.GOOS == "windows" {
			name = "go.exe"
		}
		path := filepath.Join(root, "go", "bin", name)
		if _, err := os.Stat(path); err != nil {
			return "", nil, fmt.Errorf("go sdk: %w", err)
		}
		return path, noop, nil
	}
	return fetchHostGo(vers)
}

func fetchHostGo(vers *versions.Versions) (string, func(), error) {
	p, err := platform.New(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", nil, err
	}
	urls, err := binaries.URLs(p, vers, nil)
	if err != nil {
		return "", nil, err
	}
	url := urls["go"]
	sum, err := sums.Must(url)
	if err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp("", "devlayer-hostgo-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	if p.IsWindows() {
		err = download.ZipFull(url, dir, 0, sum)
	} else {
		err = download.TarGzFull(url, dir, 0, sum)
	}
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("download host go: %w", err)
	}
	name := "go"
	if p.IsWindows() {
		name = "go.exe"
	}
	return filepath.Join(dir, "go", "bin", name), cleanup, nil
}

func glowBuildEnv(base []string, goBin, goos, goarch, proxy, gosumdb, goflags string) []string {
	goroot := filepath.Dir(filepath.Dir(goBin))
	env := withEnv(base, "GOROOT", goroot)
	env = withEnv(env, "GOOS", goos)
	env = withEnv(env, "GOARCH", goarch)
	env = withEnv(env, "CGO_ENABLED", "0")
	env = withEnv(env, "GOTOOLCHAIN", "local")
	env = withEnv(env, "GO111MODULE", "on")
	env = withEnv(env, "GOPROXY", proxy)
	env = withEnv(env, "GOSUMDB", gosumdb)
	env = withEnv(env, "GOFLAGS", goflags)
	return env
}

// buildEza compiles eza from source using cargo.
func buildEza(binDir string, vers *versions.Versions, cc []string) error {
	ver := vers.Get("EZA_VERSION")
	fmt.Println("  eza (building from source)")

	srcDir, err := os.MkdirTemp("", "devlayer-eza-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	srcRoot := filepath.Join(srcDir, "eza")
	if err := run("", nil, "git", "clone", "--depth", "1", "--branch", "v"+ver,
		"https://github.com/eza-community/eza.git", srcRoot); err != nil {
		return fmt.Errorf("eza git clone: %w", err)
	}

	cargoPath, err := exec.LookPath("cargo")
	if err != nil {
		return fmt.Errorf("find cargo: %w", err)
	}
	// CARGO_TARGET_DIR beats ~/.cargo/config.toml target-dir, so the binary
	// stays next to this clone instead of a shared cache.
	targetDir := filepath.Join(srcRoot, "target")
	clang, clangxx, sdk := compilerFields(cc)
	env := ezaCargoEnv(os.Environ(), srcRoot, runtime.GOOS, cargoPath, clang, clangxx, sdk)
	if err := run(srcRoot, env, cargoPath, ezaCargoArgs()...); err != nil {
		return fmt.Errorf("eza cargo build: %w", err)
	}
	if err := copyFile(filepath.Join(targetDir, "release", "eza"), filepath.Join(binDir, "eza")); err != nil {
		return fmt.Errorf("copy eza: %w", err)
	}
	return sealDarwin(filepath.Join(binDir, "eza"))
}

func sealDarwin(path string) error {
	if err := deadstrip.File(path); err != nil {
		return err
	}
	return deadstrip.CheckFile(path)
}

func compilerFields(env []string) (cc, cxx, sdk string) {
	for _, entry := range env {
		switch {
		case strings.HasPrefix(entry, "CC="):
			cc = strings.TrimPrefix(entry, "CC=")
		case strings.HasPrefix(entry, "CXX="):
			cxx = strings.TrimPrefix(entry, "CXX=")
		case strings.HasPrefix(entry, "SDKROOT="):
			sdk = strings.TrimPrefix(entry, "SDKROOT=")
		}
	}
	return cc, cxx, sdk
}

func ezaCargoEnv(base []string, srcRoot, goos, cargoPath, clang, clangxx, sdk string) []string {
	env := ezaBuildEnv(base, goos, cargoPath, clang, clangxx, sdk)
	return withEnv(env, "CARGO_TARGET_DIR", filepath.Join(srcRoot, "target"))
}

func ezaCargoArgs() []string {
	return []string{"build", "--release", "--features", "vendored-libgit2"}
}

func patchNvimTree(srcRoot, scriptDir string) error {
	cmakePath := filepath.Join(srcRoot, "cmake.deps", "cmake", "BuildUnibilium.cmake")
	text, err := os.ReadFile(cmakePath)
	if err != nil {
		return fmt.Errorf("read BuildUnibilium.cmake: %w", err)
	}
	next, err := patchNvimUnibilium(string(text))
	if err != nil {
		return err
	}
	if err := os.WriteFile(cmakePath, []byte(next), 0644); err != nil {
		return err
	}
	luaPath := filepath.Join(srcRoot, "src", "nvim", "CMakeLists.txt")
	luaText, err := os.ReadFile(luaPath)
	if err != nil {
		return fmt.Errorf("read nvim CMakeLists.txt: %w", err)
	}
	luaNext, err := patchNvimLuaAPI(string(luaText))
	if err != nil {
		return err
	}
	if err := os.WriteFile(luaPath, []byte(luaNext), 0644); err != nil {
		return err
	}
	patch := filepath.Join(scriptDir, "third_party", "patches", "unibilium_no_host.patch")
	if _, err := os.Stat(patch); err != nil {
		return fmt.Errorf("unibilium patch: %w", err)
	}
	return nil
}

// patchNvimUnibilium makes the unibilium subproject apply the host-search patch.
// CMAKE_EXTRA_FLAGS does not reach that subproject.
func patchNvimUnibilium(cmakeText string) (string, error) {
	const needle = "  CMAKE_ARGS ${DEPS_CMAKE_ARGS}\n"
	const repl = "  PATCH_COMMAND /usr/bin/patch -p1 -i \"$ENV{DEVLAYER_UNIBI_PATCH}\"\n  CMAKE_ARGS ${DEPS_CMAKE_ARGS} -DTERMINFO_DIRS=\n"
	if strings.Contains(cmakeText, "DEVLAYER_UNIBI_PATCH") && strings.Contains(cmakeText, "-DTERMINFO_DIRS=") {
		return cmakeText, nil
	}
	if !strings.Contains(cmakeText, needle) {
		return "", fmt.Errorf("BuildUnibilium.cmake has no CMAKE_ARGS line")
	}
	return strings.Replace(cmakeText, needle, repl, 1), nil
}

// luaAPIKeep lists Lua C API symbols that nvim does not call.
// zig cc -O2 passes -dead_strip on Darwin and --gc-sections on Linux.
// blink.cmp resolves lua_tothread when it loads the fuzzy library.
var luaAPIKeep = []string{
	"lua_atpanic",
	"lua_cpcall",
	"lua_isyieldable",
	"lua_load",
	"lua_resume",
	"lua_setallocf",
	"lua_status",
	"lua_tointegerx",
	"lua_tonumberx",
	"lua_tothread",
	"lua_upvalueid",
	"lua_upvaluejoin",
	"lua_version",
	"lua_yield",
}

func luaAPIKeepLink(macho bool) string {
	var b strings.Builder
	b.WriteString("-Wl")
	for _, name := range luaAPIKeep {
		b.WriteString(",-u,")
		if macho {
			b.WriteByte('_')
		}
		b.WriteString(name)
	}
	return b.String()
}

// patchNvimLuaAPI keeps Lua C API symbols on nvim_bin only.
// CMAKE_EXE_LINKER_FLAGS also reaches the compiler test, tee, and xxd.
func patchNvimLuaAPI(cmakeText string) (string, error) {
	const needle = "target_link_libraries(nvim_bin PRIVATE main_lib PUBLIC libuv)\n"
	if strings.Contains(cmakeText, "lua_tothread") {
		return cmakeText, nil
	}
	if !strings.Contains(cmakeText, needle) {
		return "", fmt.Errorf("src/nvim/CMakeLists.txt has no nvim_bin link line")
	}
	var b strings.Builder
	b.WriteString("if(APPLE)\n")
	b.WriteString("  target_link_options(nvim_bin PRIVATE \"")
	b.WriteString(luaAPIKeepLink(true))
	b.WriteString("\")\nelse()\n")
	b.WriteString("  target_link_options(nvim_bin PRIVATE \"")
	b.WriteString(luaAPIKeepLink(false))
	b.WriteString("\")\nendif()\n")
	return strings.Replace(cmakeText, needle, b.String()+needle, 1), nil
}

func pinNvimDeps(srcRoot string) error {
	depsPath := filepath.Join(srcRoot, "cmake.deps", "deps.txt")
	text, err := os.ReadFile(depsPath)
	if err != nil {
		return fmt.Errorf("read nvim deps: %w", err)
	}
	dir := filepath.Join(srcRoot, "devlayer-dep-archives")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	local := map[string]string{}
	for name, url := range binaries.NvimDepURLs() {
		sum, err := sums.Must(url)
		if err != nil {
			return err
		}
		dest := filepath.Join(dir, name+".tar.gz")
		if err := download.File(url, dest, sum); err != nil {
			return err
		}
		local[url] = dest
	}
	next, err := rewriteNvimDepURLs(string(text), local)
	if err != nil {
		return err
	}
	return os.WriteFile(depsPath, []byte(next), 0o644)
}

func rewriteNvimDepURLs(text string, local map[string]string) (string, error) {
	for remote, path := range local {
		if remote == "" || path == "" {
			return "", fmt.Errorf("nvim dep path is empty")
		}
		if !strings.Contains(text, remote) {
			return "", fmt.Errorf("nvim deps.txt has no %s", remote)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		text = strings.ReplaceAll(text, remote, "file://"+abs)
	}
	return text, nil
}

func nvimMakeArgs(installDir, staticPrefix string) []string {
	extra := "-DENABLE_LIBINTL=OFF -DCMAKE_IGNORE_PREFIX_PATH=/opt/homebrew"
	if staticPrefix != "" {
		extra += " -DICONV_LIBRARY=" + filepath.Join(staticPrefix, "iconv", "lib", "libiconv.a")
		extra += " -DICONV_INCLUDE_DIR=" + filepath.Join(staticPrefix, "iconv", "include")
	}
	return []string{
		"CMAKE_BUILD_TYPE=Release",
		"CMAKE_INSTALL_PREFIX=" + installDir,
		"CMAKE_EXTRA_FLAGS=" + extra,
	}
}

func htopConfigureArgs(staticPrefix string) []string {
	args := []string{"CFLAGS=-Os -DNDEBUG"}
	if staticPrefix == "" {
		return args
	}
	inc := filepath.Join(staticPrefix, "ncurses", "include")
	wide := filepath.Join(inc, "ncursesw")
	args[0] = "CFLAGS=-Os -DNDEBUG -I" + wide + " -I" + inc
	return append(args,
		"CURSES_CFLAGS=-I"+wide+" -I"+inc,
		"CURSES_LIBS="+filepath.Join(staticPrefix, "ncurses", "lib", "libncursesw.a"),
	)
}

func staticPrefix(env []string) string {
	for _, entry := range env {
		if strings.HasPrefix(entry, "DEVLAYER_STATIC=") {
			return strings.TrimPrefix(entry, "DEVLAYER_STATIC=")
		}
	}
	return ""
}

func blockHostPkgConfig(env []string) []string {
	env = dropEnv(env, "LIBGIT2_NO_VENDOR")
	env = withEnv(env, "PKG_CONFIG", "/usr/bin/false")
	env = withEnv(env, "PKG_CONFIG_PATH", "")
	return withEnv(env, "PKG_CONFIG_LIBDIR", "")
}

func dropEnv(env []string, key string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env))
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func configureErr(label, dir string, err error) error {
	tail := configLogTail(dir)
	if tail == "" {
		return fmt.Errorf("%s configure: %w", label, err)
	}
	return fmt.Errorf("%s configure: %w\n%s", label, err, tail)
}

func configLogTail(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "config.log"))
	if err != nil {
		return ""
	}
	const maxBytes = 12000
	lines := strings.Split(string(data), "\n")
	start := -1
	for i, line := range lines {
		if strings.Contains(line, "checking whether the C compiler works") {
			start = i
			break
		}
	}
	if start >= 0 {
		// Zig prints the error after the architecture list.
		end := start + 120
		if end > len(lines) {
			end = len(lines)
		}
		for i := start; i < end; i++ {
			if strings.Contains(lines[i], "Cache variables") {
				end = i
				break
			}
		}
		lines = lines[start:end]
	} else if len(lines) > 80 {
		lines = lines[len(lines)-80:]
	}
	text := strings.TrimSpace(strings.Join(lines, "\n"))
	if len(text) > maxBytes {
		text = text[len(text)-maxBytes:]
	}
	return text
}

func buildDarwinStaticLibs(vers *versions.Versions, cc []string, scriptDir string) error {
	prefix := staticPrefix(cc)
	if prefix == "" {
		return nil
	}
	urls := binaries.SourceArchiveURLs(vers)
	if err := buildAutotoolsStatic("libiconv", urls["libiconv"], filepath.Join(prefix, "iconv"), cc, []string{
		"--disable-shared",
		"--enable-static",
		"--disable-nls",
	}, false, nil); err != nil {
		return err
	}
	return buildAutotoolsStatic("ncurses", urls["ncurses"], filepath.Join(prefix, "ncurses"), cc, []string{
		"--disable-db-install",
		"--enable-widec",
		"--without-ada",
		"--without-cxx",
		"--without-debug",
		"--without-manpages",
		"--without-progs",
		"--without-shared",
		"--without-tests",
	}, true, func(src string) ([]string, func(string) error, error) {
		tic, infocmp, names, err := terminfo.Prepare(src, scriptDir)
		if err != nil {
			return nil, nil, err
		}
		return terminfo.StaticOptions(names, tic, infocmp), func(configured string) error {
			return terminfo.Check(configured, names)
		}, nil
	})
}

func buildAutotoolsStatic(label, url, prefix string, cc, configure []string, serial bool, before func(string) ([]string, func(string) error, error)) error {
	if url == "" {
		return fmt.Errorf("%s source url is empty", label)
	}
	fmt.Printf("  %s (static)\n", label)
	sum, err := sums.Must(url)
	if err != nil {
		return err
	}
	srcDir, err := os.MkdirTemp("", "devlayer-"+label+"-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)
	if err := download.TarGzFull(url, srcDir, 0, sum); err != nil {
		return fmt.Errorf("download %s: %w", label, err)
	}
	srcRoot, err := tarballRoot(srcDir, label+"-", label)
	if err != nil {
		return err
	}
	if err := settleTree(srcRoot, label); err != nil {
		return err
	}
	var extra []string
	var check func(string) error
	if before != nil {
		extra, check, err = before(srcRoot)
		if err != nil {
			return err
		}
	}
	args := append([]string{"--prefix=" + prefix}, configure...)
	args = append(args, extra...)
	env := htopBuildEnv(withCompiler(os.Environ(), cc))
	if err := run(srcRoot, env, "./configure", args...); err != nil {
		return configureErr(label, srcRoot, err)
	}
	if check != nil {
		if err := check(srcRoot); err != nil {
			return fmt.Errorf("%s terminfo: %w", label, err)
		}
	}
	makeBin := buildMakeTool(runtime.GOOS)
	jobs := fmt.Sprintf("-j%d", runtime.NumCPU())
	if serial {
		jobs = "-j1"
	}
	if err := run(srcRoot, env, makeBin, jobs); err != nil {
		return fmt.Errorf("%s build: %w", label, err)
	}
	if err := run(srcRoot, env, makeBin, "install"); err != nil {
		return fmt.Errorf("%s install: %w", label, err)
	}
	return nil
}

// buildMake compiles GNU make from source.
func buildMake(binDir string, vers *versions.Versions, cc []string) error {
	fmt.Println("  make (building from source)")

	srcDir, err := os.MkdirTemp("", "devlayer-make-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	url := binaries.SourceArchiveURLs(vers)["make"]
	sum, err := sums.Must(url)
	if err != nil {
		return err
	}
	if err := download.TarGzFull(url, srcDir, 0, sum); err != nil {
		return fmt.Errorf("download make source: %w", err)
	}
	srcRoot, err := tarballRoot(srcDir, "make-", "make")
	if err != nil {
		return err
	}
	if err := settleTree(srcRoot, "make"); err != nil {
		return err
	}

	env := htopBuildEnv(withCompiler(os.Environ(), cc))
	if err := run(srcRoot, env, "./configure", "CFLAGS=-Os -DNDEBUG"); err != nil {
		return configureErr("make", srcRoot, err)
	}
	if err := run(srcRoot, env, buildMakeTool(runtime.GOOS), fmt.Sprintf("-j%d", runtime.NumCPU())); err != nil {
		return fmt.Errorf("make build: %w", err)
	}
	if err := copyFile(filepath.Join(srcRoot, "make"), filepath.Join(binDir, "make")); err != nil {
		return fmt.Errorf("copy make: %w", err)
	}
	return sealDarwin(filepath.Join(binDir, "make"))
}

func createZigCCWrappers(binDir string) error {
	if err := os.WriteFile(filepath.Join(binDir, "cc"), []byte(zigCCUnix("cc")), 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(binDir, "c++"), []byte(zigCCUnix("c++")), 0755)
}

func zigCCUnix(tool string) string {
	return `#!/bin/sh
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

exec "$PREFIX/zig/zig" ` + tool + ` "$@"
`
}

func createZigCCWrappersWindows(binDir string) error {
	if err := os.WriteFile(filepath.Join(binDir, "cc.cmd"), []byte(zigCCWindows("cc")), 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(binDir, "c++.cmd"), []byte(zigCCWindows("c++")), 0755)
}

func zigCCWindows(tool string) string {
	return "@echo off\r\n\"%~dp0..\\zig\\zig.exe\" " + tool + " %*\r\n"
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
		found, err := stageDotfile(home, staging, rel)
		if err != nil {
			return err
		}
		if !found {
			fmt.Printf("  skipped (not found): %s\n", rel)
			continue
		}
		fmt.Printf("  %s\n", rel)
	}

	outputFile := filepath.Join(scriptDir, "devlayer-dotfiles.tar.gz")
	if err := archive.CreateTarGz(outputFile, staging); err != nil {
		return err
	}

	printSize(outputFile)
	return nil
}

// stageDotfile copies one home path into the staging tree.
// The archive stores the target of a link.
func stageDotfile(home, staging, rel string) (bool, error) {
	src := filepath.Join(home, rel)
	dst := filepath.Join(staging, rel)
	_, err := os.Stat(src)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	realSrc, err := filepath.EvalSymlinks(src)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", rel, err)
	}
	realInfo, err := os.Stat(realSrc)
	if err != nil {
		return false, err
	}
	if realInfo.IsDir() {
		if err := os.MkdirAll(dst, 0755); err != nil {
			return false, err
		}
		if err := copyDirSimple(realSrc, dst); err != nil {
			return false, fmt.Errorf("copy %s: %w", rel, err)
		}
		return true, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return false, err
	}
	if err := copyFile(realSrc, dst); err != nil {
		return false, fmt.Errorf("copy %s: %w", rel, err)
	}
	return true, nil
}

// buildNvimPlugins packages nvim plugins, treesitter parsers, and Mason LSP
// servers into a single tarball for deployment.
// Plugins come from the repo lockfile at their pinned revs. A missing lock fails.
func buildNvimPlugins(scriptDir string) error {
	lockfile := nvimplugins.RepoLockfile(scriptDir)
	if _, err := os.Stat(lockfile); err != nil {
		return fmt.Errorf("nvim pack lock: %w", err)
	}

	fmt.Println("==> Packaging nvim plugins...")

	staging, err := os.MkdirTemp("", "devlayer-nvim-plugins-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	home, _ := os.UserHomeDir()
	nvimData := filepath.Join(home, ".local", "share", "nvim")

	pluginDir := filepath.Join(staging, "site", "pack", "core", "opt")
	if err := nvimplugins.SyncPlugins(lockfile, pluginDir); err != nil {
		return fmt.Errorf("sync nvim plugins: %w", err)
	}
	entries, _ := os.ReadDir(pluginDir)
	fmt.Printf("  %d plugins\n", len(entries))

	parserDst := filepath.Join(staging, "site", "parser")
	copied, err := copyPresentDir(filepath.Join(nvimData, "site", "parser"), parserDst)
	if err != nil {
		return fmt.Errorf("copy treesitter parsers: %w", err)
	}
	if copied {
		parsers, _ := os.ReadDir(parserDst)
		fmt.Printf("  %d treesitter parsers\n", len(parsers))
	}

	if err := copyQueries(filepath.Join(nvimData, "site", "queries"), filepath.Join(staging, "site", "queries")); err != nil {
		return err
	}

	masonDst := filepath.Join(staging, "mason")
	copied, err = copyPresentDir(filepath.Join(nvimData, "mason"), masonDst)
	if err != nil {
		return fmt.Errorf("copy mason packages: %w", err)
	}
	if copied {
		pkgs, _ := os.ReadDir(filepath.Join(masonDst, "packages"))
		fmt.Printf("  %d mason packages\n", len(pkgs))
	}

	outputFile := filepath.Join(scriptDir, "devlayer-nvim-plugins.tar.gz")
	if err := archive.CreateTarGz(outputFile, staging); err != nil {
		return err
	}

	printSize(outputFile)
	return nil
}

// stampNvimSysroots puts the repo nvim config and the lua plugin pack
// into each sysroot archive in outDir.
// Config text is rung 4. The lua pack is rung 3.
// Mason bins and treesitter parsers stay in the side archive. They are rung 6.
func stampNvimSysroots(scriptDir, outDir string) error {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() || !isNvimSysrootArchive(e.Name()) {
			continue
		}
		paths = append(paths, filepath.Join(outDir, e.Name()))
	}
	if len(paths) == 0 {
		return nil
	}
	pluginTar := filepath.Join(outDir, "devlayer-nvim-plugins.tar.gz")
	if _, err := os.Stat(pluginTar); err != nil {
		return fmt.Errorf("nvim plugin pack: %w", err)
	}
	for _, path := range paths {
		fmt.Printf("==> Stamping nvim into %s\n", filepath.Base(path))
		if err := stampNvimSysroot(path, scriptDir, pluginTar); err != nil {
			return err
		}
	}
	return nil
}

func isNvimSysrootArchive(name string) bool {
	switch {
	case strings.HasPrefix(name, "devlayer-linux-") && strings.HasSuffix(name, ".tar.gz"):
		return true
	case strings.HasPrefix(name, "devlayer-darwin-") && strings.HasSuffix(name, ".tar.gz"):
		return true
	case strings.HasPrefix(name, "devlayer-windows-") && strings.HasSuffix(name, ".zip"):
		return true
	default:
		return false
	}
}

func stampNvimSysroot(archivePath, scriptDir, pluginTar string) error {
	root, err := os.MkdirTemp("", "devlayer-nvim-stamp-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)

	windows := strings.HasSuffix(archivePath, ".zip")
	if windows {
		err = archive.ExtractZip(archivePath, root)
	} else {
		err = archive.ExtractTarGz(archivePath, root)
	}
	if err != nil {
		return err
	}
	if err := placeNvimConfig(scriptDir, root); err != nil {
		return err
	}
	if err := placeNvimPack(pluginTar, root); err != nil {
		return err
	}
	if err := writeStampedNvimWrapper(root, windows); err != nil {
		return err
	}

	tmp := archivePath + ".stamp"
	defer os.Remove(tmp)
	if windows {
		err = archive.CreateZip(tmp, root)
	} else {
		err = archive.CreateTarGz(tmp, root)
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, archivePath)
}

func placeNvimPack(pluginTar, prefix string) error {
	stage, err := os.MkdirTemp("", "devlayer-nvim-pack-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := archive.ExtractTarGz(pluginTar, stage); err != nil {
		return err
	}
	src := filepath.Join(stage, "site", "pack")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("nvim plugin pack: site/pack missing")
	}
	dst := filepath.Join(prefix, "share", "devlayer", "data", "nvim", "site", "pack")
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return copyDirSimple(src, dst)
}

func writeStampedNvimWrapper(root string, windows bool) error {
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		return err
	}
	if windows {
		return createWindowsWrappers(bin, []wrapper{{
			"nvim",
			`nvim\bin\nvim.exe`,
			map[string]string{"VIMRUNTIME": `%PREFIX%\nvim\share\nvim\runtime`},
		}})
	}
	return os.WriteFile(filepath.Join(bin, "nvim"), []byte(assemble.NvimWrapper()), 0755)
}

// copyPresentDir copies a host tree. A missing tree is not an error.
func copyPresentDir(src, dst string) (bool, error) {
	if _, err := os.Stat(src); err != nil {
		return false, nil
	}
	return true, copyDirSimple(src, dst)
}

// copyQueries copies each query tree. A link is stored as its target.
// A missing query directory is not an error.
func copyQueries(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("mkdir queries: %w", err)
	}
	for _, e := range entries {
		realSrc, err := filepath.EvalSymlinks(filepath.Join(src, e.Name()))
		if err != nil {
			continue
		}
		if err := copyDirSimple(realSrc, filepath.Join(dst, e.Name())); err != nil {
			return fmt.Errorf("copy query %s: %w", e.Name(), err)
		}
	}
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

// FindScriptDir returns the source checkout.
// A checkout contains MODULE.bazel.
// An installed CLI with no checkout gets a temp dir.
func FindScriptDir(embeddedVersionsEnv, nvimPackLock string) (string, bool, error) {
	if d := os.Getenv("BUILD_WORKING_DIRECTORY"); d != "" {
		return d, false, nil
	}

	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "MODULE.bazel")); err == nil {
			return dir, false, nil
		}
	}

	cwd, err := os.Getwd()
	if err == nil {
		if _, err := os.Stat(filepath.Join(cwd, "MODULE.bazel")); err == nil {
			return cwd, false, nil
		}
	}

	tmp, err := os.MkdirTemp("", "devlayer-context-*")
	if err != nil {
		return "", false, err
	}

	if err := os.WriteFile(filepath.Join(tmp, "versions.env"), []byte(embeddedVersionsEnv), 0644); err != nil {
		return "", true, err
	}
	lockPath := nvimplugins.RepoLockfile(tmp)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return "", true, err
	}
	if err := os.WriteFile(lockPath, []byte(nvimPackLock), 0644); err != nil {
		return "", true, err
	}
	return tmp, true, nil
}

// WriteEmbeddedTree writes files for an installed CLI that has no checkout.
// A .sh file is executable. A checkout already has these files.
func WriteEmbeddedTree(dir string, files map[string]string) error {
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if strings.HasSuffix(rel, ".sh") {
			mode = 0755
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			return err
		}
	}
	return nil
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
		if w.name == "nvim" {
			path := filepath.Join(binDir, w.name)
			if err := os.WriteFile(path, []byte(assemble.NvimWrapper()), 0755); err != nil {
				return fmt.Errorf("wrapper %s: %w", w.name, err)
			}
			continue
		}
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
		if w.name == "nvim" {
			b.WriteString("set \"PREFIX=%~dp0..\"\r\n")
			b.WriteString("if exist \"%PREFIX%\\share\\devlayer\\config\\nvim\\init.lua\" if exist \"%PREFIX%\\share\\devlayer\\data\\nvim\\site\\pack\\core\\opt\\snacks.nvim\" (\r\n")
			b.WriteString("  set \"XDG_CONFIG_HOME=%PREFIX%\\share\\devlayer\\config\"\r\n")
			b.WriteString("  set \"XDG_DATA_HOME=%PREFIX%\\share\\devlayer\\data\"\r\n")
			b.WriteString(")\r\n")
		}
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
		{"lazygit", "LAZYGIT_VERSION"}, {"glow", "GLOW_VERSION"}, {"gh", "GH_VERSION"}, {"jq", "JQ_VERSION"},
		{"direnv", "DIRENV_VERSION"},
		{"nvim", "NVIM_VERSION"}, {"go", "GO_VERSION"}, {"git", "GIT_VERSION"},
		{"git-win", "GIT_WINDOWS_VERSION"}, {"zsh", "ZSH_VERSION"},
		{"htop", "HTOP_VERSION"}, {"btop", "BTOP_VERSION"}, {"dust", "DUST_VERSION"},
		{"age", "AGE_VERSION"}, {"shh", "SHH_VERSION"}, {"zig", "ZIG_VERSION"}, {"make", "MAKE_VERSION"},
		{"ncurses", "NCURSES_VERSION"}, {"batman", "BAT_EXTRAS_VERSION"},
	}
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "  %-10s %s\n", k.label, vers.Get(k.key))
	}
	return b.String()
}
