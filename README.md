# devlayer

Bring your full dev environment to any machine in layers:

1. **Tools** — the CLI and bundled binaries
2. **Runtime** — shell integration, themes, and nvim runtime
3. **Dotfiles** — the files you want on every machine
4. **Host extras** — optional layers like SSH config

This is an **opinionated** tool. It ships a curated set of tools, expects a specific shell setup, and makes choices so you don't have to. If you want a menu of optional plugins and hand-tuned package manager steps, this isn't that. devlayer is for shipping a working stack fast.

## Quick start

**Linux / macOS — install the CLI:**
```bash
curl -fsSL https://raw.githubusercontent.com/stefanpenner/devlayer/master/scripts/install.sh | bash
source ~/.profile
```

**Windows (PowerShell) — install the CLI:**
```powershell
irm https://raw.githubusercontent.com/stefanpenner/devlayer/master/scripts/install.ps1 | iex
```

**Then pick a path:**

```bash
# Local machine
devlayer install

# Or build a bundle first, then install it locally
devlayer build --os darwin
devlayer install

# Or push the full stack to a remote Linux host over SSH
devlayer build
devlayer push nas
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

## Mental model

devlayer is easiest to use if you treat it like a stack with clear layers:

| Layer | What it contains | Where it lands |
|---|---|---|
| Tools | `fd`, `rg`, `jq`, `git`, `zsh`, `nvim`, `go`, `zig`, etc. | `$DEVLAYER_PREFIX/` |
| Runtime | wrappers, themes, shell integration, bundled runtime files | `$DEVLAYER_PREFIX/` and `~/.local/share/` |
| Dotfiles | your synced config files | `$HOME/` |
| Host extras | optional machine-sensitive config like SSH | `$HOME/` |

The sweet spot is:

1. keep the **tool layer** the same everywhere
2. keep the **runtime layer** opinionated
3. keep the **dotfile layer** explicit
4. keep the **SSH layer** config-only

## Common commands

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

## Recommended first run

```bash
# 1. Install the CLI
curl -fsSL https://raw.githubusercontent.com/stefanpenner/devlayer/master/scripts/install.sh | bash

# 2. Create your sync config
mkdir -p ~/.config/devlayer
$EDITOR ~/.config/devlayer/config.toml

# 3. Build and install locally
devlayer build --os darwin
devlayer install

# 4. Check what landed
devlayer ls
devlayer status
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

`push` uses your existing SSH setup. If `nas` works in `ssh nas`, it should work in `devlayer push nas`.

## Layering your config

devlayer can bundle your dotfiles and pre-downloaded nvim plugins alongside the tool bundle. Create `~/.config/devlayer/config.toml`:

```toml
[dotfiles]
sync = [
  # shell
  ".bash_profile",
  ".zshrc",
  ".config/zsh",
  ".p10k.zsh",

  # terminal + editor
  ".config/ghostty",
  ".config/nvim",
  ".config/opencode",
  ".tmux.conf",
  ".gitconfig",

  # optional SSH layer: config only, never keys
  ".ssh/config",
  ".ssh/config.d",
]
```

### SSH layer

The safe rule is:

- sync **SSH config**
- do **not** sync private keys
- do **not** sync agent sockets
- usually do **not** sync `known_hosts`

If you keep SSH config in a separate repo such as `stefanpenner/ssh`, the easiest pattern is:

1. materialize that repo into `~/.ssh/config` and optionally `~/.ssh/config.d/`
2. add only those config files to devlayer
3. keep keys outside devlayer

Example `~/.ssh/config`:

```sshconfig
Host *
  IdentityAgent /path/to/agent.sock

Include ~/.ssh/config.d/*.conf
```

This keeps SSH as a **layer on top of the base stack**, instead of mixing secrets into the bundle.

## Dotfiles & nvim plugins

If you use LazyVim (or any lazy.nvim setup), devlayer reads your `lazy-lock.json` and bundles all locally-installed plugins. On push, nvim starts fully loaded — no first-launch download.

```bash
devlayer build --os linux         # Builds tools + dotfiles + nvim plugins
devlayer push nas                 # Deploys all layers
```

Four layers, one command:
1. **Tools** → `$DEVLAYER_PREFIX/` (binaries)
2. **Runtime** → wrappers, themes, shared support files
3. **Dotfiles** → `$HOME/` (your config files)
4. **Nvim plugins** → `~/.local/share/nvim/lazy/` (pre-downloaded)

## Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `DEVLAYER_PREFIX` | `~/.local` (unix) / `%LOCALAPPDATA%\devlayer` (Windows) | Install location for all tools |
| `XDG_DATA_HOME` | `~/.local/share` | Plugin/data search path (used by zsh config) |

### What to sync

Good fits:

- shell config
- terminal config
- editor config
- git config
- SSH config files

Bad fits:

- private keys
- machine-local secrets
- caches
- large generated directories

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
