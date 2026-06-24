package platform

import (
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		os, arch    string
		wantErr     bool
		wantRust    string
		wantGoArch  string
		wantGeneric string
		wantExe     string
		wantBundle  string
		wantNvimOS  string
		wantJqOS    string
	}{
		{
			name: "linux amd64", os: "linux", arch: "x86_64",
			wantRust: "x86_64-unknown-linux-musl", wantGoArch: "amd64",
			wantGeneric: "x86_64", wantExe: "", wantBundle: "tar.gz",
			wantNvimOS: "linux", wantJqOS: "linux",
		},
		{
			name: "linux arm64", os: "linux", arch: "aarch64",
			wantRust: "aarch64-unknown-linux-musl", wantGoArch: "arm64",
			wantGeneric: "arm64", wantExe: "", wantBundle: "tar.gz",
			wantNvimOS: "linux", wantJqOS: "linux",
		},
		{
			name: "linux go-style arch", os: "linux", arch: "amd64",
			wantRust: "x86_64-unknown-linux-musl", wantGoArch: "amd64",
			wantGeneric: "x86_64", wantExe: "", wantBundle: "tar.gz",
			wantNvimOS: "linux", wantJqOS: "linux",
		},
		{
			name: "darwin arm64", os: "darwin", arch: "arm64",
			wantRust: "aarch64-apple-darwin", wantGoArch: "arm64",
			wantGeneric: "arm64", wantExe: "", wantBundle: "tar.gz",
			wantNvimOS: "macos", wantJqOS: "macos",
		},
		{
			name: "darwin x86_64", os: "darwin", arch: "x86_64",
			wantRust: "x86_64-apple-darwin", wantGoArch: "amd64",
			wantGeneric: "x86_64", wantExe: "", wantBundle: "tar.gz",
			wantNvimOS: "macos", wantJqOS: "macos",
		},
		{
			name: "windows amd64", os: "windows", arch: "x86_64",
			wantRust: "x86_64-pc-windows-msvc", wantGoArch: "amd64",
			wantGeneric: "x86_64", wantExe: ".exe", wantBundle: "zip",
			wantNvimOS: "win64", wantJqOS: "windows",
		},
		{
			name: "windows arm64", os: "windows", arch: "arm64",
			wantRust: "aarch64-pc-windows-msvc", wantGoArch: "arm64",
			wantGeneric: "arm64", wantExe: ".exe", wantBundle: "zip",
			wantNvimOS: "win-arm64", wantJqOS: "windows",
		},
		{
			name: "unsupported os", os: "freebsd", arch: "x86_64",
			wantErr: true,
		},
		{
			name: "unsupported arch", os: "linux", arch: "mips",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.os, tt.arch)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if p.RustTarget != tt.wantRust {
				t.Errorf("RustTarget = %q, want %q", p.RustTarget, tt.wantRust)
			}
			if p.GoArch != tt.wantGoArch {
				t.Errorf("GoArch = %q, want %q", p.GoArch, tt.wantGoArch)
			}
			if p.ArchGeneric != tt.wantGeneric {
				t.Errorf("ArchGeneric = %q, want %q", p.ArchGeneric, tt.wantGeneric)
			}
			if p.ExeSuffix != tt.wantExe {
				t.Errorf("ExeSuffix = %q, want %q", p.ExeSuffix, tt.wantExe)
			}
			if p.BundleExt != tt.wantBundle {
				t.Errorf("BundleExt = %q, want %q", p.BundleExt, tt.wantBundle)
			}
			if p.NvimOS != tt.wantNvimOS {
				t.Errorf("NvimOS = %q, want %q", p.NvimOS, tt.wantNvimOS)
			}
			if p.JqOS != tt.wantJqOS {
				t.Errorf("JqOS = %q, want %q", p.JqOS, tt.wantJqOS)
			}
		})
	}
}

func TestRustTargetFor(t *testing.T) {
	tests := []struct {
		name    string
		os      string
		arch    string
		project string
		want    string
	}{
		{"ripgrep linux x86_64", "linux", "x86_64", "ripgrep", "x86_64-unknown-linux-musl"},
		{"ripgrep linux aarch64", "linux", "aarch64", "ripgrep", "aarch64-unknown-linux-gnu"},
		{"delta linux aarch64", "linux", "aarch64", "delta", "aarch64-unknown-linux-gnu"},
		{"fd linux aarch64", "linux", "aarch64", "fd", "aarch64-unknown-linux-musl"},
		{"ripgrep darwin arm64", "darwin", "arm64", "ripgrep", "aarch64-apple-darwin"},
		{"ripgrep windows x86_64", "windows", "x86_64", "ripgrep", "x86_64-pc-windows-msvc"},
		{"dust darwin arm64", "darwin", "arm64", "dust", "x86_64-apple-darwin"},
		{"dust darwin x86_64", "darwin", "x86_64", "dust", "x86_64-apple-darwin"},
		{"dust linux x86_64", "linux", "x86_64", "dust", "x86_64-unknown-linux-musl"},
		{"dust linux aarch64", "linux", "aarch64", "dust", "aarch64-unknown-linux-musl"},
		// eza only ships a windows-gnu build (no msvc); other platforms use the default target.
		{"eza windows x86_64", "windows", "x86_64", "eza", "x86_64-pc-windows-gnu"},
		{"eza windows arm64", "windows", "arm64", "eza", "aarch64-pc-windows-gnu"},
		{"eza linux x86_64", "linux", "x86_64", "eza", "x86_64-unknown-linux-musl"},
		{"eza darwin arm64", "darwin", "arm64", "eza", "aarch64-apple-darwin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.os, tt.arch)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got := p.RustTargetFor(tt.project)
			if got != tt.want {
				t.Errorf("RustTargetFor(%q) = %q, want %q", tt.project, got, tt.want)
			}
		})
	}
}

func TestSkipTool(t *testing.T) {
	tests := []struct {
		name string
		os   string
		tool string
		want bool
	}{
		{"zsh on linux", "linux", "zsh", false},
		{"zsh on windows", "windows", "zsh", true},
		{"git on windows", "windows", "git", false},
		{"batman on windows", "windows", "batman", true},
		{"htop on windows", "windows", "htop", true},
		{"btop on windows", "windows", "btop", true},
		{"btop on linux", "linux", "btop", false},
		{"fzf on windows", "windows", "fzf", false},
		{"nvim on windows", "windows", "nvim", false},
		{"fd on linux", "linux", "fd", false},
		{"batman on darwin", "darwin", "batman", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.os, "x86_64")
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got := p.SkipTool(tt.tool)
			if got != tt.want {
				t.Errorf("SkipTool(%q) = %v, want %v", tt.tool, got, tt.want)
			}
		})
	}
}

func TestIsWindows(t *testing.T) {
	tests := []struct {
		os   string
		want bool
	}{
		{"linux", false},
		{"darwin", false},
		{"windows", true},
	}
	for _, tt := range tests {
		p, _ := New(tt.os, "x86_64")
		if got := p.IsWindows(); got != tt.want {
			t.Errorf("IsWindows() for %s = %v, want %v", tt.os, got, tt.want)
		}
	}
}

func TestArchiveExtensions(t *testing.T) {
	tests := []struct {
		os       string
		wantRust string
		wantFzf  string
		wantNvim string
		wantGo   string
	}{
		{"linux", "tar.gz", "tar.gz", "tar.gz", "tar.gz"},
		{"darwin", "tar.gz", "tar.gz", "tar.gz", "tar.gz"},
		{"windows", "zip", "zip", "zip", "zip"},
	}

	for _, tt := range tests {
		t.Run(tt.os, func(t *testing.T) {
			p, _ := New(tt.os, "x86_64")
			if got := p.RustArchiveExt(); got != tt.wantRust {
				t.Errorf("RustArchiveExt() = %q, want %q", got, tt.wantRust)
			}
			if got := p.FzfArchiveExt(); got != tt.wantFzf {
				t.Errorf("FzfArchiveExt() = %q, want %q", got, tt.wantFzf)
			}
			if got := p.NvimArchiveExt(); got != tt.wantNvim {
				t.Errorf("NvimArchiveExt() = %q, want %q", got, tt.wantNvim)
			}
			if got := p.GoArchiveExt(); got != tt.wantGo {
				t.Errorf("GoArchiveExt() = %q, want %q", got, tt.wantGo)
			}
		})
	}
}

func TestDockerPlatform(t *testing.T) {
	tests := []struct {
		arch string
		want string
	}{
		{"x86_64", "linux/amd64"},
		{"arm64", "linux/arm64"},
	}
	for _, tt := range tests {
		p, _ := New("linux", tt.arch)
		if p.DockerPlatform != tt.want {
			t.Errorf("DockerPlatform for %s = %q, want %q", tt.arch, p.DockerPlatform, tt.want)
		}
	}
}
