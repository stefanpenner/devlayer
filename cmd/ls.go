package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/list"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/stefanpenner/devlayer/internal/config"
	"github.com/stefanpenner/devlayer/internal/nvimplugins"
	"github.com/stefanpenner/devlayer/internal/platform"
)

var (
	heading = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")). // bright blue
		MarginBottom(1)

	subtext = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")) // dim gray

	checkMark = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")). // green
			SetString("✓")

	crossMark = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")). // red
			SetString("✗")

	runtimeBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")). // cyan
			Bold(true)

	dimText = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))
)

type statusRow struct {
	Name    string
	Present bool
}

// bundledBinaries is the product catalog. ls never dumps $prefix/bin.
func bundledBinaries(osName string) []string {
	names := []string{
		"age", "age-keygen", "bat", "batman", "btop", "cc", "c++",
		"delta", "devlayer", "direnv", "dust", "eza", "fd", "fzf",
		"gh", "git", "go", "gofmt", "htop", "jq", "lazygit", "make",
		"nvim", "rg", "zig", "zsh",
	}
	if osName != "windows" {
		names = append(names, "ls")
	}
	p, err := platform.New(osName, "x86_64")
	if err != nil {
		sort.Strings(names)
		return names
	}
	out := names[:0]
	for _, n := range names {
		if p.SkipTool(n) {
			continue
		}
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func toolStatus(binDir string, catalog []string) []statusRow {
	rows := make([]statusRow, 0, len(catalog))
	for _, name := range catalog {
		rows = append(rows, statusRow{Name: name, Present: binPresent(binDir, name)})
	}
	return rows
}

func binPresent(binDir, name string) bool {
	candidates := []string{filepath.Join(binDir, name)}
	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			filepath.Join(binDir, name+".exe"),
			filepath.Join(binDir, name+".cmd"),
		)
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func pluginStatus(lockNames []string, pluginDir string) []statusRow {
	names := append([]string(nil), lockNames...)
	sort.Strings(names)
	rows := make([]statusRow, 0, len(names))
	for _, name := range names {
		_, err := os.Stat(filepath.Join(pluginDir, name))
		rows = append(rows, statusRow{Name: name, Present: err == nil})
	}
	return rows
}

func renderStatusTable(rows []statusRow, cols int) string {
	cells := make([]string, len(rows))
	for i, r := range rows {
		mark := checkMark.String()
		if !r.Present {
			mark = crossMark.String()
		}
		cells[i] = mark + " " + r.Name
	}
	if cols < 1 {
		cols = 1
	}
	n := (len(cells) + cols - 1) / cols
	t := table.New().
		Border(lipgloss.HiddenBorder()).
		StyleFunc(func(row, col int) lipgloss.Style {
			return lipgloss.NewStyle().PaddingRight(2)
		})
	for r := range n {
		row := make([]string, cols)
		for c := range cols {
			idx := c*n + r
			if idx < len(cells) {
				row[c] = cells[idx]
			}
		}
		t.Row(row...)
	}
	return t.String()
}

// Ls lists bundled tools, configured dotfiles, and lockfile nvim plugins.
func Ls() error {
	prefix := defaultPrefix()

	if err := lsTools(prefix); err != nil {
		return err
	}
	fmt.Println()
	lsDotfiles()
	fmt.Println()
	lsNvimPlugins()

	return nil
}

// lsTools lists the catalog under the prefix — not every file in bin/.
func lsTools(prefix string) error {
	fmt.Println(heading.Render("Tools") + " " + subtext.Render(filepath.Join(prefix, "bin")))

	binDir := filepath.Join(prefix, "bin")
	rows := toolStatus(binDir, bundledBinaries(runtime.GOOS))
	fmt.Println(renderStatusTable(rows, 4))

	// Show bundled subdirectories
	subdirs := []string{"go", "git", "zsh", "nvim", "zig", "share"}
	var present []string
	for _, d := range subdirs {
		if info, err := os.Stat(filepath.Join(prefix, d)); err == nil && info.IsDir() {
			present = append(present, runtimeBadge.Render(d))
		}
	}
	if len(present) > 0 {
		fmt.Println(subtext.Render("  Bundled runtimes: ") + strings.Join(present, subtext.Render(", ")))
	}

	return nil
}

// lsDotfiles lists synced dotfiles from the config.
func lsDotfiles() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println(heading.Render("Dotfiles"))
		fmt.Printf("  %s\n", dimText.Render("error loading config: "+err.Error()))
		return
	}

	fmt.Println(heading.Render("Dotfiles") + " " + subtext.Render(config.Path()))

	if len(cfg.Dotfiles.Sync) == 0 {
		fmt.Println(dimText.Render("  (none configured)"))
		return
	}

	home, _ := os.UserHomeDir()
	items := make([]any, 0, len(cfg.Dotfiles.Sync))
	for _, rel := range cfg.Dotfiles.Sync {
		full := filepath.Join(home, rel)
		marker := checkMark.String()
		if _, err := os.Stat(full); os.IsNotExist(err) {
			marker = crossMark.String() + " " + dimText.Render("missing")
		}
		items = append(items, rel+" "+marker)
	}

	l := list.New(items...).Enumerator(list.Dash)
	fmt.Println(l)
}

// lsNvimPlugins lists installed nvim plugins from the vim.pack directory.
func lsNvimPlugins() {
	pluginDir := nvimplugins.LocalPluginDir()
	fmt.Println(heading.Render("Nvim Plugins") + " " + subtext.Render(pluginDir))

	plugins, err := nvimplugins.ParseLockfile(nvimplugins.LockfilePath())
	if err != nil {
		fmt.Println(dimText.Render("  (no lockfile)"))
		return
	}
	names := make([]string, 0, len(plugins))
	for _, p := range plugins {
		names = append(names, p.Name)
	}
	if len(names) == 0 {
		fmt.Println(dimText.Render("  (none in lockfile)"))
		return
	}

	rows := pluginStatus(names, pluginDir)
	fmt.Println(renderStatusTable(rows, 3))
	fmt.Println(subtext.Render(fmt.Sprintf("  %d plugins", len(names))))
}
