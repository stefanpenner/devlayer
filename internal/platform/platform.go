package platform

import "fmt"

// Platform holds normalized OS/arch information and target triples.
type Platform struct {
	OS             string // "linux", "darwin", or "windows"
	Arch           string // original arch string
	RustArch       string // "x86_64" or "aarch64"
	GoArch         string // "amd64" or "arm64"
	ArchGeneric    string // "x86_64" or "arm64"
	RustTarget     string // e.g. "x86_64-unknown-linux-musl"
	RustTargetGNU  string // for ripgrep/delta aarch64 fallback
	LazygitOS      string // "Linux", "Darwin", or "Windows"
	NvimOS         string // "linux", "macos", "win64", "win-arm64"
	JqOS           string // "linux", "macos", or "windows"
	ZigOS          string // "linux", "macos", or "windows"
	DockerPlatform string // e.g. "linux/amd64"
	ExeSuffix      string // "" on unix, ".exe" on windows
	BundleExt      string // "tar.gz" or "zip"
}

// New creates a Platform with all fields normalized.
func New(os, arch string) (*Platform, error) {
	p := &Platform{OS: os, Arch: arch}

	switch arch {
	case "x86_64", "amd64":
		p.RustArch = "x86_64"
		p.GoArch = "amd64"
		p.ArchGeneric = "x86_64"
	case "aarch64", "arm64":
		p.RustArch = "aarch64"
		p.GoArch = "arm64"
		p.ArchGeneric = "arm64"
	default:
		return nil, fmt.Errorf("unsupported arch: %s", arch)
	}

	switch os {
	case "linux":
		p.RustTarget = p.RustArch + "-unknown-linux-musl"
		p.RustTargetGNU = p.RustArch + "-unknown-linux-gnu"
		p.LazygitOS = "Linux"
		p.NvimOS = "linux"
		p.JqOS = "linux"
		p.ZigOS = "linux"
		p.BundleExt = "tar.gz"
	case "darwin":
		p.RustTarget = p.RustArch + "-apple-darwin"
		p.RustTargetGNU = p.RustTarget // not needed on Mac
		p.LazygitOS = "Darwin"
		p.NvimOS = "macos"
		p.JqOS = "macos"
		p.ZigOS = "macos"
		p.BundleExt = "tar.gz"
	case "windows":
		p.RustTarget = p.RustArch + "-pc-windows-msvc"
		p.RustTargetGNU = p.RustTarget
		p.LazygitOS = "Windows"
		p.JqOS = "windows"
		p.ZigOS = "windows"
		p.ExeSuffix = ".exe"
		p.BundleExt = "zip"
		// nvim uses non-standard naming on Windows
		if p.GoArch == "amd64" {
			p.NvimOS = "win64"
		} else {
			p.NvimOS = "win-arm64"
		}
	default:
		return nil, fmt.Errorf("unsupported OS: %s", os)
	}

	p.DockerPlatform = "linux/" + p.GoArch

	return p, nil
}

// IsWindows returns true if the platform targets Windows.
func (p *Platform) IsWindows() bool {
	return p.OS == "windows"
}

// SkipTool returns true if a tool is not available on this platform.
func (p *Platform) SkipTool(name string) bool {
	if p.OS == "windows" {
		switch name {
		case "zsh", "batman", "htop", "btop":
			return true
		}
	}
	return false
}

// RustTargetFor returns the appropriate rust target triple for a project.
// ripgrep and delta don't ship aarch64 musl builds on Linux.
func (p *Platform) RustTargetFor(project string) string {
	switch project {
	case "ripgrep", "delta":
		if p.OS == "linux" && p.RustArch == "aarch64" {
			return p.RustTargetGNU
		}
		return p.RustTarget
	case "dust":
		// dust doesn't ship aarch64-apple-darwin; use x86_64 via Rosetta
		if p.OS == "darwin" && p.RustArch == "aarch64" {
			return "x86_64-apple-darwin"
		}
		return p.RustTarget
	case "eza":
		// eza only ships a windows-gnu build (no msvc)
		if p.OS == "windows" {
			return p.RustArch + "-pc-windows-gnu"
		}
		return p.RustTarget
	default:
		return p.RustTarget
	}
}

// RustArchiveExt returns the archive extension for Rust tool releases.
// Windows uses .zip, everything else uses .tar.gz.
func (p *Platform) RustArchiveExt() string {
	if p.OS == "windows" {
		return "zip"
	}
	return "tar.gz"
}

// FzfArchiveExt returns the archive extension for fzf releases.
func (p *Platform) FzfArchiveExt() string {
	if p.OS == "windows" {
		return "zip"
	}
	return "tar.gz"
}

// LazygitArchiveExt returns the archive extension for lazygit releases.
func (p *Platform) LazygitArchiveExt() string {
	if p.OS == "windows" {
		return "zip"
	}
	return "tar.gz"
}

// NvimArchiveName returns the nvim archive base name (before extension).
func (p *Platform) NvimArchiveName(version string) string {
	if p.OS == "windows" {
		return fmt.Sprintf("nvim-%s", p.NvimOS)
	}
	return fmt.Sprintf("nvim-%s-%s", p.NvimOS, p.ArchGeneric)
}

// NvimArchiveExt returns the archive extension for nvim releases.
func (p *Platform) NvimArchiveExt() string {
	if p.OS == "windows" {
		return "zip"
	}
	return "tar.gz"
}

// GoArchiveExt returns the archive extension for Go SDK releases.
func (p *Platform) GoArchiveExt() string {
	if p.OS == "windows" {
		return "zip"
	}
	return "tar.gz"
}

