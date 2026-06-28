# devlayer

Layer your dev environment onto any machine. Tools, config, and nvim plugins — one command.

This is an **opinionated** tool. It ships a curated set of tools, expects a specific shell setup, and makes choices so you don't have to. If you want to pick and choose individual components, this isn't for you — devlayer gives you the whole stack or nothing.

All binaries are statically linked — no system dependencies required. devlayer itself is a single Go binary.

## Quick start

**Linux / macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/stefanpenner/devlayer/master/scripts/install.sh | bash
source ~/.profile
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/stefanpenner/devlayer/master/scripts/install.ps1 | iex
```

Or download the CLI directly and build:

```bash
# Download the CLI (pick your platform)
curl -fsSL https://github.com/stefanpenner/devlayer/releases/latest/download/devlayer-darwin-arm64 -o devlayer
chmod +x devlayer

# Build and install the tool bundle
./devlayer build --os darwin
./devlayer install
```

## Usage

```bash
devlayer build                    # Build linux bundle (Docker)
devlayer build --os darwin        # Build macOS bundle
devlayer build --os darwin --nvim-head  # Build with nvim from HEAD
devlayer build --os windows       # Build Windows bundle
devlayer push nas                 # Deploy to remote host via SSH
devlayer status                   # Check installed versions locally
devlayer status nas               # Check installed versions on host
devlayer upgrade                  # Download and install latest release
devlayer install                  # Install bundle locally
devlayer ls                       # List installed tools, dotfiles, and plugins
devlayer clean                    # Remove build artifacts
devlayer version                  # Print devlayer version
devlayer versions                 # Print bundled tool versions
```

## What's included

| Tool | Linux | macOS | Windows |
|------|:-----:|:-----:|:-------:|
| zsh | yes | yes | — |
| git | yes | yes | yes |
| nvim | yes | yes | yes |
| go | yes | yes | yes |
| fzf | yes | yes | yes |
| fd | yes | yes | yes |
| bat | yes | yes | yes |
| rg (ripgrep) | yes | yes | yes |
| eza (ls) | yes | yes | yes |
| delta | yes | yes | yes |
| jq | yes | yes | yes |
| direnv | yes | yes | yes |
| lazygit | yes | yes | yes |
| htop | yes | yes | — |
| btop | yes | yes | — |
| dust | yes | yes | yes |
| age | yes | yes | yes |
| zig (cc/c++) | yes | yes | yes |
| make | yes | yes | — |
| batman | yes | yes | — |
| devlayer | yes | yes | yes |

Also bundles zsh plugins (autosuggestions, fast-syntax-highlighting, history-substring-search, powerlevel10k) and fzf shell integration on Linux/macOS.

### Portability

On **Linux**, all binaries are statically linked against musl libc — they run on any Linux distribution with no shared library dependencies.

On **macOS**, nvim, htop, and btop are compiled from source for best portability. Rust and Go tools are downloaded as pre-built releases (already statically linked). All macOS binaries are **best-effort hermetic** — they link against `libSystem.dylib` (always present) but have no other external dependencies. Apple does not support fully static linking, so this is the best achievable.

On **Windows**, binaries are downloaded from upstream releases. Git uses [MinGit](https://github.com/git-for-windows/git) — a portable, self-contained distribution. They don't require package managers but depend on system DLLs.

Some tools require supporting files at runtime (git needs `libexec/`, nvim needs `share/nvim/runtime/`, go needs its SDK, zsh needs function files). These are handled transparently via wrapper scripts in `bin/` that set the correct environment variables (`GIT_EXEC_PATH`, `VIMRUNTIME`, `GOROOT`, `FPATH`) before exec'ing the real binary — no manual configuration needed beyond PATH.

All other tools are single-binary and fully self-contained (on Linux, statically linked; on macOS/Windows, best-effort).

## Shell integration

Wrapper scripts in `bin/` handle `GOROOT`, `GIT_EXEC_PATH`, `VIMRUNTIME`, and `FPATH` automatically, so you only need to add `bin/` to your PATH.

**Bash** (`~/.bashrc` or `~/.bash_profile`):
```bash
export PATH="$HOME/.local/bin:$PATH"

claude() {
  command claude --ax-screen-reader "$@"
}

copilot() {
  command copilot --screen-reader --no-mouse --plain-diff "$@"
}
```

**Zsh** (`~/.zshrc`):
```zsh
export PATH="$HOME/.local/bin:$PATH"

# Bundled plugins (optional)
source "$HOME/.local/share/zsh-autosuggestions/zsh-autosuggestions.zsh"
source "$HOME/.local/share/zsh-fast-syntax-highlighting/fast-syntax-highlighting.plugin.zsh"
source "$HOME/.local/share/zsh-history-substring-search/zsh-history-substring-search.zsh"
source "$HOME/.local/share/powerlevel10k/powerlevel10k.zsh-theme"

# fzf keybindings and completion (optional)
source "$HOME/.local/share/fzf/key-bindings.zsh"
source "$HOME/.local/share/fzf/completion.zsh"

# Accessibility wrappers
claude() {
  command claude --ax-screen-reader "$@"
}

copilot() {
  command copilot --screen-reader --no-mouse --plain-diff "$@"
}
```

**Windows (PowerShell profile)**:
```powershell
$env:PATH = "$env:LOCALAPPDATA\devlayer\bin;$env:PATH"
```

**direnv** — if using direnv, also add the hook:
```bash
# bash
eval "$(direnv hook bash)"

# zsh
eval "$(direnv hook zsh)"
```

## Deploy to a remote host

```bash
devlayer build                    # Builds linux bundle via Docker
devlayer push nas                 # Deploys to nas:~/.local/
devlayer status nas               # Verify everything works
```

## Dotfiles & nvim plugins

devlayer can bundle your dotfiles and pre-downloaded nvim plugins alongside the tool bundle. Create `~/.config/devlayer/config.toml`:

```toml
[dotfiles]
sync = [
  ".config/ghostty",
  ".config/nvim",
  ".config/opencode",
  ".bash_profile",
  ".zshrc",
  ".tmux.conf",
  ".gitconfig",
  ".p10k.zsh",
]
```

If you use LazyVim (or any lazy.nvim setup), devlayer reads your `lazy-lock.json` and bundles all locally-installed plugins. On push, nvim starts fully loaded — no first-launch download.

```bash
devlayer build --os linux         # Builds tools + dotfiles + nvim plugins
devlayer push nas                 # Deploys all three layers
```

Three layers, one command:
1. **Tools** → `$DEVLAYER_PREFIX/` (binaries)
2. **Dotfiles** → `$HOME/` (your config files)
3. **Nvim plugins** → `~/.local/share/nvim/lazy/` (pre-downloaded)

## Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `DEVLAYER_PREFIX` | `~/.local` (unix) / `%LOCALAPPDATA%\devlayer` (Windows) | Install location for all tools |
| `XDG_DATA_HOME` | `~/.local/share` | Plugin/data search path (used by zsh config) |

## Updating tool versions

Edit `versions.env`, commit, push, and create a release:

```bash
vim versions.env
git commit -am "bump fd to 10.5.0"
gh release create v0.2.0
```

A weekly GHA workflow also checks for new upstream versions and opens PRs automatically.

## Supply chain security

- All GHA actions pinned by commit SHA
- SLSA build provenance via `actions/attest-build-provenance`
- SHA256 checksums for every release artifact
- Dependabot keeps GHA actions updated
- Weekly automated checks for new tool versions (opens PRs)
- All tool versions pinned in [`versions.env`](versions.env)

## Building from source

```bash
bazel build //:devlayer                # Build the devlayer CLI
bazel build //third_party/btop         # Build btop from source
bazel build //third_party/make:gnumake # Build GNU make from source
bazel test //...                       # Run all tests
```

Requires [Bazel](https://bazel.build/) (or [Bazelisk](https://github.com/bazelbuild/bazelisk)). The build uses `hermetic_cc_toolchain` (zig-based) for reproducible C/C++ compilation and `rules_foreign_cc` for cmake/autotools projects.
