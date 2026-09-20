#!/bin/bash
# Download pre-built static/musl binaries for devlayer.
# Usage: download-binaries.sh <output-dir> [os] [arch]
#   os defaults to current (linux/darwin)
#   arch defaults to current machine
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=../versions.env
source "$SCRIPT_DIR/../versions.env"

OUT="${1:?usage: download-binaries.sh <output-dir> [os] [arch]}"
OS="${2:-$(uname -s | tr '[:upper:]' '[:lower:]')}"
ARCH="${3:-$(uname -m)}"

mkdir -p "$OUT/bin"

# Normalize architecture
case "$ARCH" in
  x86_64|amd64)  RUST_ARCH=x86_64  GOARCH=amd64  ARCH_GENERIC=x86_64 ;;
  aarch64|arm64)  RUST_ARCH=aarch64 GOARCH=arm64  ARCH_GENERIC=arm64  ;;
  *) echo "Unsupported arch: $ARCH" >&2; exit 1 ;;
esac

# OS-specific target triples & naming
case "$OS" in
  linux)
    RUST_TARGET="${RUST_ARCH}-unknown-linux-musl"
    # delta doesn't ship aarch64 musl builds
    RUST_TARGET_GNU="${RUST_ARCH}-unknown-linux-gnu"
    LAZYGIT_OS=Linux
    NVIM_OS=linux
    JQ_OS=linux
    ;;
  darwin)
    RUST_TARGET="${RUST_ARCH}-apple-darwin"
    RUST_TARGET_GNU="${RUST_TARGET}"  # not needed on Mac
    LAZYGIT_OS=Darwin
    NVIM_OS=macos
    JQ_OS=macos
    ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Some Rust projects don't publish every musl/darwin target
rust_target_for() {
  local project=$1
  case "$project" in
    delta)
      # delta still has no aarch64 musl build on Linux
      if [ "$OS" = "linux" ] && [ "$RUST_ARCH" = "aarch64" ]; then
        echo "$RUST_TARGET_GNU"
      else
        echo "$RUST_TARGET"
      fi
      ;;
    *) echo "$RUST_TARGET" ;;
  esac
}

dl() {
  local url=$1 dest=$2
  echo "  $(basename "$dest")"
  curl -fsSL "$url" -o "$dest"
}

dl_tar() {
  local url=$1 binary=$2
  local tmp; tmp=$(mktemp -d)
  echo "  $binary"
  curl -fsSL "$url" | tar xz -C "$tmp"
  find "$tmp" -name "$binary" -type f -exec cp {} "$OUT/bin/$binary" \;
  rm -rf "$tmp"
}

dl_tbz() {
  local url=$1 binary=$2
  local tmp; tmp=$(mktemp -d)
  echo "  $binary"
  curl -fsSL "$url" | tar xj -C "$tmp"
  find "$tmp" -name "$binary" -type f -exec cp {} "$OUT/bin/$binary" \;
  rm -rf "$tmp"
}

echo "==> Downloading binaries ($OS/$ARCH)"

# --- Rust tools (musl for Linux, apple-darwin for Mac) ---
dl_tar "https://github.com/sharkdp/fd/releases/download/v${FD_VERSION}/fd-v${FD_VERSION}-${RUST_TARGET}.tar.gz" fd
dl_tar "https://github.com/sharkdp/bat/releases/download/v${BAT_VERSION}/bat-v${BAT_VERSION}-${RUST_TARGET}.tar.gz" bat
dl_tar "https://github.com/BurntSushi/ripgrep/releases/download/${RG_VERSION}/ripgrep-${RG_VERSION}-$(rust_target_for ripgrep).tar.gz" rg
dl_tar "https://github.com/dandavison/delta/releases/download/${DELTA_VERSION}/delta-${DELTA_VERSION}-$(rust_target_for delta).tar.gz" delta
dl_tar "https://github.com/bootandy/dust/releases/download/v${DUST_VERSION}/dust-v${DUST_VERSION}-$(rust_target_for dust).tar.gz" dust

# --- Go tools (static) ---
dl_tar "https://github.com/junegunn/fzf/releases/download/v${FZF_VERSION}/fzf-${FZF_VERSION}-${OS}_${GOARCH}.tar.gz" fzf
dl_tar "https://github.com/jesseduffield/lazygit/releases/download/v${LAZYGIT_VERSION}/lazygit_${LAZYGIT_VERSION}_${LAZYGIT_OS}_${ARCH_GENERIC}.tar.gz" lazygit

echo "  gh"
if [ "$OS" = "darwin" ]; then
  tmp=$(mktemp -d)
  curl -fsSL "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_macOS_${GOARCH}.zip" -o "$tmp/gh.zip"
  unzip -q "$tmp/gh.zip" -d "$tmp"
  find "$tmp" -name gh -type f -exec cp {} "$OUT/bin/gh" \;
  rm -rf "$tmp"
else
  dl_tar "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${GOARCH}.tar.gz" gh
fi

# --- age (encryption tool — two binaries) ---
AGE_URL="https://github.com/FiloSottile/age/releases/download/v${AGE_VERSION}/age-v${AGE_VERSION}-${OS}-${GOARCH}.tar.gz"
dl_tar "$AGE_URL" age
dl_tar "$AGE_URL" age-keygen

# --- Single-binary tools ---
dl "https://github.com/direnv/direnv/releases/download/v${DIRENV_VERSION}/direnv.${OS}-${GOARCH}" "$OUT/bin/direnv"
dl "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/jq-${JQ_OS}-${GOARCH}" "$OUT/bin/jq"

# --- bat-extras (batman is a shell script — portable) ---
tmp=$(mktemp -d)
curl -fsSL "https://github.com/eth-p/bat-extras/releases/download/v${BAT_EXTRAS_VERSION}/bat-extras-${BAT_EXTRAS_VERSION}.zip" -o "$tmp/bat-extras.zip"
unzip -q "$tmp/bat-extras.zip" -d "$tmp/bat-extras"
cp "$tmp/bat-extras/bin/batman" "$OUT/bin/batman"
rm -rf "$tmp"
echo "  batman"

# --- Neovim (self-contained tarball) ---
if [ "${SKIP_NVIM:-}" != "1" ]; then
  echo "  nvim"
  nvim_tmp=$(mktemp -d)
  curl -fsSL "https://github.com/neovim/neovim/releases/download/v${NVIM_VERSION}/nvim-${NVIM_OS}-${ARCH_GENERIC}.tar.gz" | tar xz -C "$nvim_tmp"
  mv "$nvim_tmp"/nvim-* "$OUT/nvim"
  rm -rf "$nvim_tmp"
fi

# --- Go SDK ---
echo "  go"
curl -fsSL "https://go.dev/dl/go${GO_VERSION}.${OS}-${GOARCH}.tar.gz" | tar xz -C "$OUT"

# --- Zig (C/C++ compiler) ---
echo "  zig"
ZIG_OS="$OS"
[ "$ZIG_OS" = "darwin" ] && ZIG_OS="macos"
zig_tmp=$(mktemp -d)
curl -fsSL "https://ziglang.org/download/${ZIG_VERSION}/zig-${RUST_ARCH}-${ZIG_OS}-${ZIG_VERSION}.tar.xz" | tar xJ -C "$zig_tmp"
mv "$zig_tmp"/zig-*/ "$OUT/zig"
rm -rf "$zig_tmp"

# --- fzf shell integration (from source repo) ---
echo "  fzf shell integration"
fzf_tmp=$(mktemp -d)
curl -fsSL "https://github.com/junegunn/fzf/archive/refs/tags/v${FZF_VERSION}.tar.gz" | tar xz -C "$fzf_tmp"
mkdir -p "$OUT/share/fzf"
cp "$fzf_tmp"/fzf-*/shell/key-bindings.zsh "$OUT/share/fzf/"
cp "$fzf_tmp"/fzf-*/shell/completion.zsh "$OUT/share/fzf/"
rm -rf "$fzf_tmp"

# --- Zsh plugins (shell scripts — portable) ---
mkdir -p "$OUT/share"

dl_plugin() {
  local name=$1 url=$2
  echo "  $name"
  local tmp; tmp=$(mktemp -d)
  curl -fsSL "$url" | tar xz -C "$tmp"
  # GitHub tarballs extract to repo-tag/, flatten to just the name
  mv "$tmp"/*/ "$OUT/share/$name"
  rm -rf "$tmp"
}

dl_plugin zsh-autosuggestions \
  "https://github.com/zsh-users/zsh-autosuggestions/archive/refs/tags/${ZSH_AUTOSUGGESTIONS_VERSION}.tar.gz"
dl_plugin zsh-fast-syntax-highlighting \
  "https://github.com/zdharma-continuum/fast-syntax-highlighting/archive/refs/tags/${FAST_SYNTAX_HIGHLIGHTING_VERSION}.tar.gz"
dl_plugin zsh-history-substring-search \
  "https://github.com/zsh-users/zsh-history-substring-search/archive/refs/tags/${ZSH_HISTORY_SUBSTRING_SEARCH_VERSION}.tar.gz"
dl_plugin powerlevel10k \
  "https://github.com/romkatv/powerlevel10k/archive/refs/tags/${POWERLEVEL10K_VERSION}.tar.gz"

chmod +x "$OUT/bin"/*
echo "==> Done: $(ls "$OUT/bin" | wc -l | tr -d ' ') binaries + nvim + go + plugins"
