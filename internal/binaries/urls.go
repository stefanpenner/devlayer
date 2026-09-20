// Package binaries resolves pinned tool URLs and fetches them into a layout.
//
// resolve platform → list URLs → fetch into out → chmod bin
package binaries

import (
	"fmt"
	"strings"

	"github.com/stefanpenner/devlayer/internal/platform"
	"github.com/stefanpenner/devlayer/internal/versions"
)

// URLs returns tool-name → download URL for this platform and pins.
// Omits skipped tools and platform-unavailable tools (platform.SkipTool).
func URLs(p *platform.Platform, vers *versions.Versions, skip map[string]bool) (map[string]string, error) {
	u := make(map[string]string)
	put := func(name, url string) {
		if skip[name] || p.SkipTool(name) {
			return
		}
		u[name] = url
	}

	put("fd", fdURL(p, vers.Get("FD_VERSION")))
	put("bat", batURL(p, vers.Get("BAT_VERSION")))
	put("rg", rgURL(p, vers.Get("RG_VERSION")))
	put("delta", deltaURL(p, vers.Get("DELTA_VERSION")))
	put("dust", dustURL(p, vers.Get("DUST_VERSION")))

	if p.IsWindows() {
		put("eza", ezaURL(p, vers.Get("EZA_VERSION")))
	}

	put("fzf", fzfURL(p, vers.Get("FZF_VERSION")))
	put("lazygit", lazygitURL(p, vers.Get("LAZYGIT_VERSION")))

	age := ageURL(p, vers.Get("AGE_VERSION"))
	put("age", age)
	put("age-keygen", age)

	put("direnv", direnvURL(p, vers.Get("DIRENV_VERSION")))
	put("jq", jqURL(p, vers.Get("JQ_VERSION")))
	put("batman", batmanURL(vers.Get("BAT_EXTRAS_VERSION")))
	put("nvim", nvimURL(p, vers.Get("NVIM_VERSION")))
	put("go", goSDKURL(p, vers.Get("GO_VERSION")))
	put("zig", zigURL(p, vers.Get("ZIG_VERSION")))

	if p.IsWindows() {
		put("git", minGitURL(p, vers.Get("GIT_WINDOWS_VERSION")))
	}

	if !p.IsWindows() {
		put("fzf-shell", fzfShellURL(vers.Get("FZF_VERSION")))
		put("zsh-autosuggestions", pluginURL("zsh-users/zsh-autosuggestions", vers.Get("ZSH_AUTOSUGGESTIONS_VERSION")))
		put("zsh-fast-syntax-highlighting", pluginURL("zdharma-continuum/fast-syntax-highlighting", vers.Get("FAST_SYNTAX_HIGHLIGHTING_VERSION")))
		put("zsh-history-substring-search", pluginURL("zsh-users/zsh-history-substring-search", vers.Get("ZSH_HISTORY_SUBSTRING_SEARCH_VERSION")))
		put("powerlevel10k", pluginURL("romkatv/powerlevel10k", vers.Get("POWERLEVEL10K_VERSION")))
	}

	return u, nil
}

func githubRelease(repo, tag, asset string) string {
	return "https://github.com/" + repo + "/releases/download/" + tag + "/" + asset
}

func rustURL(p *platform.Platform, repo, tag, base string) string {
	return githubRelease(repo, tag, base+"."+p.RustArchiveExt())
}

func fdURL(p *platform.Platform, ver string) string {
	return rustURL(p, "sharkdp/fd", "v"+ver, "fd-v"+ver+"-"+p.RustTarget)
}

func batURL(p *platform.Platform, ver string) string {
	return rustURL(p, "sharkdp/bat", "v"+ver, "bat-v"+ver+"-"+p.RustTarget)
}

func rgURL(p *platform.Platform, ver string) string {
	return rustURL(p, "BurntSushi/ripgrep", ver, "ripgrep-"+ver+"-"+p.RustTargetFor("ripgrep"))
}

func deltaURL(p *platform.Platform, ver string) string {
	return rustURL(p, "dandavison/delta", ver, "delta-"+ver+"-"+p.RustTargetFor("delta"))
}

func dustURL(p *platform.Platform, ver string) string {
	return rustURL(p, "bootandy/dust", "v"+ver, "dust-v"+ver+"-"+p.RustTargetFor("dust"))
}

func ezaURL(p *platform.Platform, ver string) string {
	return githubRelease("eza-community/eza", "v"+ver, "eza.exe_"+p.RustTargetFor("eza")+".zip")
}

func fzfURL(p *platform.Platform, ver string) string {
	asset := fmt.Sprintf("fzf-%s-%s_%s.%s", ver, p.OS, p.GoArch, p.FzfArchiveExt())
	return githubRelease("junegunn/fzf", "v"+ver, asset)
}

func lazygitURL(p *platform.Platform, ver string) string {
	asset := fmt.Sprintf("lazygit_%s_%s_%s.%s", ver, p.LazygitOS, p.ArchGeneric, p.LazygitArchiveExt())
	return githubRelease("jesseduffield/lazygit", "v"+ver, asset)
}

func ageURL(p *platform.Platform, ver string) string {
	ext := "tar.gz"
	if p.IsWindows() {
		ext = "zip"
	}
	asset := fmt.Sprintf("age-v%s-%s-%s.%s", ver, p.OS, p.GoArch, ext)
	return githubRelease("FiloSottile/age", "v"+ver, asset)
}

func direnvURL(p *platform.Platform, ver string) string {
	return githubRelease("direnv/direnv", "v"+ver, fmt.Sprintf("direnv.%s-%s", p.OS, p.GoArch))
}

func jqURL(p *platform.Platform, ver string) string {
	tag := "jq-" + ver
	if p.IsWindows() {
		return githubRelease("jqlang/jq", tag, fmt.Sprintf("jq-windows-%s.exe", p.GoArch))
	}
	return githubRelease("jqlang/jq", tag, fmt.Sprintf("jq-%s-%s", p.JqOS, p.GoArch))
}

func batmanURL(ver string) string {
	return githubRelease("eth-p/bat-extras", "v"+ver, "bat-extras-"+ver+".zip")
}

func nvimURL(p *platform.Platform, ver string) string {
	return githubRelease("neovim/neovim", "v"+ver, p.NvimArchiveName(ver)+"."+p.NvimArchiveExt())
}

func goSDKURL(p *platform.Platform, ver string) string {
	return fmt.Sprintf("https://go.dev/dl/go%s.%s-%s.%s", ver, p.OS, p.GoArch, p.GoArchiveExt())
}

func zigURL(p *platform.Platform, ver string) string {
	ext := "tar.xz"
	if p.IsWindows() {
		ext = "zip"
	}
	return fmt.Sprintf("https://ziglang.org/download/%s/zig-%s-%s-%s.%s", ver, p.RustArch, p.ZigOS, ver, ext)
}

func minGitURL(p *platform.Platform, ver string) string {
	parts := strings.Split(ver, ".")
	patch := parts[len(parts)-1]
	base := strings.Join(parts[:len(parts)-1], ".")
	arch := "64-bit"
	if p.GoArch == "arm64" {
		arch = "arm64"
	}
	return fmt.Sprintf("https://github.com/git-for-windows/git/releases/download/v%s.windows.%s/MinGit-%s-%s.zip",
		base, patch, ver, arch)
}

func fzfShellURL(ver string) string {
	return fmt.Sprintf("https://github.com/junegunn/fzf/archive/refs/tags/v%s.tar.gz", ver)
}

func pluginURL(repo, ver string) string {
	return fmt.Sprintf("https://github.com/%s/archive/refs/tags/%s.tar.gz", repo, ver)
}
