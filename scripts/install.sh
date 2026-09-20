#!/bin/bash
# Devlayer installer — downloads and extracts the hermetic tool bundle.
#
# Usage:
#   curl -fsSL https://github.com/stefanpenner/devlayer/releases/latest/download/install.sh | bash
#
# Options (env vars):
#   DEVLAYER_VERSION=v0.1.0   Pin to a specific release (default: latest)
#   DEVLAYER_DIR=$HOME/.local  Install directory (default: ~/.local)
#   DEVLAYER_REPO=user/repo   Override repo (default: stefanpenner/devlayer)
#
set -euo pipefail

REPO="${DEVLAYER_REPO:-stefanpenner/devlayer}"
VERSION="${DEVLAYER_VERSION:-latest}"
INSTALL_DIR="${DEVLAYER_DIR:-$HOME/.local}"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

# Normalize arch
case "$ARCH" in
  x86_64|amd64)  ARCH=x86_64 ;;
  aarch64|arm64)
    case "$OS" in
      linux)  ARCH=aarch64 ;;
      darwin) ARCH=arm64 ;;
    esac
    ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

ASSET="devlayer-${OS}-${ARCH}.tar.gz"
CHECKSUM_ASSET="devlayer-${OS}-${ARCH}.tar.gz.sha256"

if [ "$VERSION" = "latest" ]; then
  BASE_URL="https://github.com/${REPO}/releases/latest/download"
else
  BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
fi

echo "devlayer: installing ${VERSION} for ${OS}/${ARCH}"
echo "  target: ${INSTALL_DIR}"

# Download
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "  downloading ${ASSET}..."
curl -fsSL "${BASE_URL}/${ASSET}" -o "${TMP}/${ASSET}"

# Verify checksum if available
if curl -fsSL "${BASE_URL}/${CHECKSUM_ASSET}" -o "${TMP}/${CHECKSUM_ASSET}" 2>/dev/null; then
  echo "  verifying checksum..."
  cd "$TMP"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c "${CHECKSUM_ASSET}"
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 -c "${CHECKSUM_ASSET}"
  else
    echo "  warning: no sha256sum available, skipping verification"
  fi
  cd - >/dev/null
fi

# Extract
echo "  extracting..."
mkdir -p "${INSTALL_DIR}"
tar xzf "${TMP}/${ASSET}" -C "${INSTALL_DIR}"

echo ""
echo "devlayer installed to ${INSTALL_DIR}"
echo ""

BIN_DIR="${INSTALL_DIR}/bin"
PROFILE="${HOME}/.profile"
if ! grep -qs 'devlayer' "${PROFILE}" 2>/dev/null; then
  echo "  adding ${BIN_DIR} to ${PROFILE}"
  {
    echo ""
    echo "# devlayer"
    echo "export PATH=\"${BIN_DIR}:\$PATH\""
  } >> "${PROFILE}"
fi
export PATH="${BIN_DIR}:${PATH}"

echo "Open a new shell (or: source ${PROFILE})"
echo "Then:  devlayer init    # writes ~/.config/devlayer/config.toml"
echo "       devlayer doctor"
