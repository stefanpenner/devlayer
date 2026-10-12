package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCreateUnixWrappers(t *testing.T) {
	tests := []struct {
		name         string
		wrapper      wrapper
		wantContains []string
	}{
		{
			name: "git wrapper",
			wrapper: wrapper{"git", "git/bin/git", map[string]string{
				"GIT_EXEC_PATH": "$PREFIX/git/libexec/git-core",
			}},
			wantContains: []string{
				"#!/bin/sh",
				`PREFIX="$(cd "$(dirname "$0")/.." && pwd)"`,
				"GIT_EXEC_PATH=",
				"git/libexec/git-core",
				`exec "$PREFIX/git/bin/git" "$@"`,
			},
		},
		{
			name: "nvim wrapper",
			wrapper: wrapper{"nvim", "nvim/bin/nvim", map[string]string{
				"VIMRUNTIME": "$PREFIX/nvim/share/nvim/runtime",
			}},
			wantContains: []string{
				"#!/bin/sh",
				"VIMRUNTIME=",
				"nvim/share/nvim/runtime",
				`[ -f "$PREFIX/share/devlayer/config/nvim/init.lua" ]`,
				`[ -d "$PREFIX/share/devlayer/data/nvim/site/pack/core/opt/snacks.nvim" ]`,
				`export XDG_CONFIG_HOME="$PREFIX/share/devlayer/config"`,
				`export XDG_DATA_HOME="$PREFIX/share/devlayer/data"`,
				`exec "$PREFIX/nvim/bin/nvim" "$@"`,
			},
		},
		{
			name: "go wrapper",
			wrapper: wrapper{"go", "go/bin/go", map[string]string{
				"GOROOT": "$PREFIX/go",
			}},
			wantContains: []string{
				"#!/bin/sh",
				"GOROOT=",
				`exec "$PREFIX/go/bin/go" "$@"`,
			},
		},
		{
			name: "gofmt wrapper",
			wrapper: wrapper{"gofmt", "go/bin/gofmt", map[string]string{
				"GOROOT": "$PREFIX/go",
			}},
			wantContains: []string{
				"#!/bin/sh",
				"GOROOT=",
				`exec "$PREFIX/go/bin/gofmt" "$@"`,
			},
		},
		{
			name: "zsh wrapper with FPATH append",
			wrapper: wrapper{"zsh", "zsh/bin/zsh", map[string]string{
				"FPATH": "$PREFIX/zsh/share/zsh/functions",
			}},
			wantContains: []string{
				"#!/bin/sh",
				"FPATH=",
				"${FPATH:+:$FPATH}",
				`exec "$PREFIX/zsh/bin/zsh" "$@"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := createUnixWrappers(dir, []wrapper{tt.wrapper}); err != nil {
				t.Fatalf("createUnixWrappers: %v", err)
			}

			content, err := os.ReadFile(filepath.Join(dir, tt.wrapper.name))
			if err != nil {
				t.Fatalf("read wrapper: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(string(content), want) {
					t.Errorf("wrapper missing %q\ngot:\n%s", want, content)
				}
			}

			// Verify file is executable (skip on Windows — no Unix permissions)
			if runtime.GOOS != "windows" {
				info, _ := os.Stat(filepath.Join(dir, tt.wrapper.name))
				if info.Mode()&0111 == 0 {
					t.Error("wrapper is not executable")
				}
			}
		})
	}
}

func TestCreateWindowsWrappers(t *testing.T) {
	tests := []struct {
		name         string
		wrapper      wrapper
		wantContains []string
	}{
		{
			name: "git wrapper",
			wrapper: wrapper{"git", `git\cmd\git.exe`, map[string]string{
				"GIT_EXEC_PATH": `%PREFIX%\git\mingw64\libexec\git-core`,
			}},
			wantContains: []string{
				"@echo off",
				`set "GIT_EXEC_PATH=%~dp0..\git\mingw64\libexec\git-core"`,
				`git\cmd\git.exe`,
			},
		},
		{
			name: "go wrapper",
			wrapper: wrapper{"go", `go\bin\go.exe`, map[string]string{
				"GOROOT": `%PREFIX%\go`,
			}},
			wantContains: []string{
				"@echo off",
				`set "GOROOT=%~dp0..\go"`,
				`go\bin\go.exe`,
			},
		},
		{
			name: "nvim wrapper",
			wrapper: wrapper{"nvim", `nvim\bin\nvim.exe`, map[string]string{
				"VIMRUNTIME": `%PREFIX%\nvim\share\nvim\runtime`,
			}},
			wantContains: []string{
				"@echo off",
				`set "VIMRUNTIME=%~dp0..\nvim\share\nvim\runtime"`,
				`XDG_CONFIG_HOME=%PREFIX%\share\devlayer\config`,
				`snacks.nvim`,
				`nvim\bin\nvim.exe`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := createWindowsWrappers(dir, []wrapper{tt.wrapper}); err != nil {
				t.Fatalf("createWindowsWrappers: %v", err)
			}

			content, err := os.ReadFile(filepath.Join(dir, tt.wrapper.name+".cmd"))
			if err != nil {
				t.Fatalf("read wrapper: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(string(content), want) {
					t.Errorf("wrapper missing %q\ngot:\n%s", want, content)
				}
			}
		})
	}
}
