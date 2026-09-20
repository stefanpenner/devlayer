#!/bin/bash
# Check upstream tool versions and rewrite versions.env in place.
# Usage: scripts/check-updates.sh
# When GITHUB_OUTPUT is set (Actions), also writes has_updates=true/false.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# shellcheck source=../versions.env
source versions.env

UPDATES=""
UPDATES_FILE="${UPDATES_FILE:-/tmp/devlayer-updates.txt}"

is_release_ver() {
  case "$1" in
    ''|*[!A-Za-z0-9._-]*) return 1 ;;
    *rc*|*RC*|*alpha*|*beta*|*nightly*|*snapshot*) return 1 ;;
  esac
  return 0
}

strip_prefix() {
  local tag=$1
  tag=${tag#v}
  tag=${tag#jq-}
  tag=${tag#go}
  printf '%s\n' "$tag"
}

latest_release() {
  local repo=$1
  local tag
  tag=$(gh api "repos/${repo}/releases/latest" --jq '.tag_name' 2>/dev/null || true)
  tag=$(strip_prefix "$tag")
  if is_release_ver "$tag"; then
    printf '%s\n' "$tag"
  fi
}

latest_git() {
  gh api "repos/git/git/tags?per_page=40" --jq '.[].name' 2>/dev/null \
    | sed 's/^v//' \
    | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' \
    | head -1 || true
}

latest_git_windows() {
  local tag
  tag=$(gh api "repos/git-for-windows/git/releases/latest" --jq '.tag_name' 2>/dev/null || true)
  # v2.55.0.windows.5 → 2.55.0.5
  tag=$(printf '%s\n' "$tag" | sed -E 's/^v//; s/\.windows\././')
  if is_release_ver "$tag"; then
    printf '%s\n' "$tag"
  fi
}

latest_go() {
  local ver
  ver=$(curl -fsSL 'https://go.dev/dl/?mode=json' | jq -r '.[0].version' 2>/dev/null || true)
  ver=$(strip_prefix "$ver")
  if is_release_ver "$ver"; then
    printf '%s\n' "$ver"
  fi
}

latest_zig() {
  local ver
  ver=$(curl -fsSL 'https://ziglang.org/download/index.json' \
    | jq -r 'keys | map(select(test("^[0-9]+\\.[0-9]+\\.[0-9]+$"))) | sort_by(split(".") | map(tonumber)) | last' \
    2>/dev/null || true)
  if is_release_ver "$ver"; then
    printf '%s\n' "$ver"
  fi
}

latest_zsh() {
  local ver
  ver=$(curl -fsSL 'https://www.zsh.org/pub/' \
    | grep -oE 'zsh-[0-9]+(\.[0-9]+)+\.tar\.xz' \
    | sed 's/^zsh-//; s/\.tar\.xz$//' \
    | sort -t. -k1,1n -k2,2n -k3,3n \
    | tail -1 || true)
  if is_release_ver "$ver"; then
    printf '%s\n' "$ver"
  fi
}

latest_gnu() {
  local project=$1
  local ver
  ver=$(curl -fsSL "https://ftp.gnu.org/gnu/${project}/" \
    | grep -oE "${project}-[0-9]+(\.[0-9]+)+\.tar\.gz" \
    | sed "s/^${project}-//; s/\.tar\.gz$//" \
    | sort -t. -k1,1n -k2,2n -k3,3n \
    | tail -1 || true)
  if is_release_ver "$ver"; then
    printf '%s\n' "$ver"
  fi
}

set_version() {
  local key=$1 val=$2
  awk -v key="$key" -v val="$val" '
    $0 ~ "^" key "=" { print key "=" val; next }
    { print }
  ' versions.env > versions.env.tmp
  mv versions.env.tmp versions.env
}

check_update() {
  local key=$1 current=$2 latest=$3
  if [ -z "$latest" ] || [ "$current" = "$latest" ]; then
    return 0
  fi
  UPDATES="${UPDATES}${key}: ${current} → ${latest}\n"
  set_version "${key}_VERSION" "$latest"
}

check_update FZF "$FZF_VERSION" "$(latest_release junegunn/fzf)"
check_update FD "$FD_VERSION" "$(latest_release sharkdp/fd)"
check_update BAT "$BAT_VERSION" "$(latest_release sharkdp/bat)"
check_update EZA "$EZA_VERSION" "$(latest_release eza-community/eza)"
check_update RG "$RG_VERSION" "$(latest_release BurntSushi/ripgrep)"
check_update DELTA "$DELTA_VERSION" "$(latest_release dandavison/delta)"
check_update LAZYGIT "$LAZYGIT_VERSION" "$(latest_release jesseduffield/lazygit)"
check_update BAT_EXTRAS "$BAT_EXTRAS_VERSION" "$(latest_release eth-p/bat-extras)"
check_update JQ "$JQ_VERSION" "$(latest_release jqlang/jq)"
check_update DIRENV "$DIRENV_VERSION" "$(latest_release direnv/direnv)"
check_update NVIM "$NVIM_VERSION" "$(latest_release neovim/neovim)"
check_update GIT "$GIT_VERSION" "$(latest_git)"
check_update GIT_WINDOWS "$GIT_WINDOWS_VERSION" "$(latest_git_windows)"
check_update HTOP "$HTOP_VERSION" "$(latest_release htop-dev/htop)"
check_update BTOP "$BTOP_VERSION" "$(latest_release aristocratos/btop)"
check_update DUST "$DUST_VERSION" "$(latest_release bootandy/dust)"
check_update AGE "$AGE_VERSION" "$(latest_release FiloSottile/age)"
check_update GO "$GO_VERSION" "$(latest_go)"
check_update ZIG "$ZIG_VERSION" "$(latest_zig)"
check_update ZSH "$ZSH_VERSION" "$(latest_zsh)"
check_update MAKE "$MAKE_VERSION" "$(latest_gnu make)"
check_update NCURSES "$NCURSES_VERSION" "$(latest_gnu ncurses)"

if [ -n "$UPDATES" ]; then
  printf '%b' "$UPDATES" > "$UPDATES_FILE"
  echo "==> Updates found:"
  cat "$UPDATES_FILE"
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "has_updates=true" >> "$GITHUB_OUTPUT"
  fi
else
  : > "$UPDATES_FILE"
  echo "==> All tools are up to date"
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "has_updates=false" >> "$GITHUB_OUTPUT"
  fi
fi
