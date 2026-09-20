package binaries

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/stefanpenner/devlayer/internal/download"
	"github.com/stefanpenner/devlayer/internal/platform"
	"github.com/stefanpenner/devlayer/internal/versions"
)

// Fetch downloads the pinned tool set into out.
// Layout:
//
//	out/bin/<tool>, out/nvim/, out/go/, out/zig/, out/share/fzf/, out/share/<plugin>/
func Fetch(out string, p *platform.Platform, vers *versions.Versions, skip map[string]bool) error {
	fmt.Printf("==> Downloading binaries (%s/%s)\n", p.OS, p.Arch)

	binDir := filepath.Join(out, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	urls, err := URLs(p, vers, skip)
	if err != nil {
		return err
	}

	if err := fetchRust(binDir, p, urls); err != nil {
		return err
	}
	if err := fetchGoTools(binDir, p, urls); err != nil {
		return err
	}
	if err := fetchAge(binDir, p, urls); err != nil {
		return err
	}
	if err := fetchSingles(binDir, p, urls); err != nil {
		return err
	}
	if err := fetchBatman(binDir, vers, urls); err != nil {
		return err
	}
	if err := fetchNvim(out, p, urls); err != nil {
		return err
	}
	if err := fetchGoSDK(out, p, urls); err != nil {
		return err
	}
	if err := fetchZig(out, p, urls); err != nil {
		return err
	}
	if err := fetchMinGit(out, urls); err != nil {
		return err
	}
	if err := fetchFzfShell(out, urls); err != nil {
		return err
	}
	if err := fetchZshPlugins(out, urls); err != nil {
		return err
	}

	if err := chmodBin(binDir); err != nil {
		return err
	}

	printDone(binDir, p)
	return nil
}

func fetchRust(binDir string, p *platform.Platform, urls map[string]string) error {
	ext := p.RustArchiveExt()
	for _, name := range []string{"fd", "bat", "rg", "delta", "dust"} {
		url, ok := urls[name]
		if !ok {
			continue
		}
		if err := extractBin(url, binDir, name+p.ExeSuffix, ext); err != nil {
			return fmt.Errorf("download %s: %w", name, err)
		}
	}

	if url, ok := urls["eza"]; ok {
		if err := download.ZipBinary(url, binDir, "eza.exe"); err != nil {
			return fmt.Errorf("download eza: %w", err)
		}
	}
	return nil
}

func fetchGoTools(binDir string, p *platform.Platform, urls map[string]string) error {
	if url, ok := urls["fzf"]; ok {
		if err := extractBin(url, binDir, "fzf"+p.ExeSuffix, p.FzfArchiveExt()); err != nil {
			return fmt.Errorf("download fzf: %w", err)
		}
	}

	if url, ok := urls["lazygit"]; ok {
		if err := extractBin(url, binDir, "lazygit"+p.ExeSuffix, p.LazygitArchiveExt()); err != nil {
			return fmt.Errorf("download lazygit: %w", err)
		}
	}
	return nil
}

func fetchAge(binDir string, p *platform.Platform, urls map[string]string) error {
	ext := "tar.gz"
	if p.IsWindows() {
		ext = "zip"
	}
	for _, name := range []string{"age", "age-keygen"} {
		url, ok := urls[name]
		if !ok {
			continue
		}
		if err := extractBin(url, binDir, name+p.ExeSuffix, ext); err != nil {
			return fmt.Errorf("download %s: %w", name, err)
		}
	}
	return nil
}

func fetchSingles(binDir string, p *platform.Platform, urls map[string]string) error {
	if url, ok := urls["direnv"]; ok {
		dest := filepath.Join(binDir, "direnv"+p.ExeSuffix)
		if err := download.File(url, dest); err != nil {
			return fmt.Errorf("download direnv: %w", err)
		}
	}

	if url, ok := urls["jq"]; ok {
		dest := filepath.Join(binDir, "jq"+p.ExeSuffix)
		if err := download.File(url, dest); err != nil {
			return fmt.Errorf("download jq: %w", err)
		}
	}
	return nil
}

func fetchBatman(binDir string, vers *versions.Versions, urls map[string]string) error {
	url, ok := urls["batman"]
	if !ok {
		return nil
	}

	ver := vers.Get("BAT_EXTRAS_VERSION")
	dest := filepath.Join(binDir, "batman")
	primary := "bat-extras-" + ver + "/bin/batman"
	if err := download.ZipFiles(url, map[string]string{primary: dest}); err != nil {
		return fmt.Errorf("download batman: %w", err)
	}
	if _, err := os.Stat(dest); err != nil {
		if err := download.ZipFiles(url, map[string]string{"bin/batman": dest}); err != nil {
			return fmt.Errorf("download batman: %w", err)
		}
	}

	fmt.Println("  batman")
	return nil
}

func fetchNvim(out string, p *platform.Platform, urls map[string]string) error {
	url, ok := urls["nvim"]
	if !ok {
		return nil
	}

	fmt.Println("  nvim")
	if err := extractFull(url, filepath.Join(out, "nvim"), 1, p.NvimArchiveExt()); err != nil {
		return fmt.Errorf("download nvim: %w", err)
	}
	return nil
}

func fetchGoSDK(out string, p *platform.Platform, urls map[string]string) error {
	url, ok := urls["go"]
	if !ok {
		return nil
	}

	fmt.Println("  go")
	if err := extractFull(url, out, 0, p.GoArchiveExt()); err != nil {
		return fmt.Errorf("download go: %w", err)
	}
	return nil
}

func fetchZig(out string, p *platform.Platform, urls map[string]string) error {
	url, ok := urls["zig"]
	if !ok {
		return nil
	}

	fmt.Println("  zig")
	ext := "tar.xz"
	if p.IsWindows() {
		ext = "zip"
	}
	if err := extractFull(url, filepath.Join(out, "zig"), 1, ext); err != nil {
		return fmt.Errorf("download zig: %w", err)
	}
	return nil
}

func fetchMinGit(out string, urls map[string]string) error {
	url, ok := urls["git"]
	if !ok {
		return nil
	}

	fmt.Println("  git (MinGit)")
	if err := download.ZipFull(url, filepath.Join(out, "git"), 0); err != nil {
		return fmt.Errorf("download git: %w", err)
	}
	return nil
}

func fetchFzfShell(out string, urls map[string]string) error {
	url, ok := urls["fzf-shell"]
	if !ok {
		return nil
	}

	fmt.Println("  fzf shell integration")
	dest := filepath.Join(out, "share", "fzf")
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	tmp, err := os.MkdirTemp("", "devlayer-fzf-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if err := download.TarGzFull(url, tmp, 0); err != nil {
		return fmt.Errorf("download fzf shell: %w", err)
	}

	entries, err := os.ReadDir(tmp)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		shellDir := filepath.Join(tmp, e.Name(), "shell")
		if _, err := os.Stat(shellDir); err != nil {
			continue
		}
		for _, name := range []string{"key-bindings.zsh", "completion.zsh"} {
			src := filepath.Join(shellDir, name)
			if _, err := os.Stat(src); err != nil {
				continue
			}
			if err := copyFile(src, filepath.Join(dest, name)); err != nil {
				return err
			}
		}
		break
	}
	return nil
}

func fetchZshPlugins(out string, urls map[string]string) error {
	for _, name := range []string{
		"zsh-autosuggestions",
		"zsh-fast-syntax-highlighting",
		"zsh-history-substring-search",
		"powerlevel10k",
	} {
		url, ok := urls[name]
		if !ok {
			continue
		}
		fmt.Printf("  %s\n", name)
		if err := download.TarGzToDir(url, filepath.Join(out, "share", name)); err != nil {
			return fmt.Errorf("download %s: %w", name, err)
		}
	}
	return nil
}

func extractBin(url, dir, name, ext string) error {
	if ext == "zip" {
		return download.ZipBinary(url, dir, name)
	}
	return download.TarGzBinary(url, dir, name)
}

func extractFull(url, dir string, strip int, ext string) error {
	switch ext {
	case "zip":
		return download.ZipFull(url, dir, strip)
	case "tar.xz":
		return download.TarXzFull(url, dir, strip)
	default:
		return download.TarGzFull(url, dir, strip)
	}
}

func chmodBin(binDir string) error {
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.Chmod(filepath.Join(binDir, e.Name()), 0755); err != nil {
			return err
		}
	}
	return nil
}

func printDone(binDir string, p *platform.Platform) {
	entries, _ := os.ReadDir(binDir)
	fmt.Printf("==> Done: %d binaries + nvim + go", len(entries))
	if !p.IsWindows() {
		fmt.Print(" + plugins")
	}
	fmt.Println()
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
