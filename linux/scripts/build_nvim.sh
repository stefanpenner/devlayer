#!/bin/sh
# Build neovim as a static binary.
set -euo pipefail

exec 3>&1  # save stdout for final tar output
exec 1>&2  # redirect all build output to stderr

# Expects NVIM_VERSION env var.
git clone --depth 1 --branch "v${NVIM_VERSION}" https://github.com/neovim/neovim.git
cd neovim
make CMAKE_BUILD_TYPE=Release \
     CMAKE_EXTRA_FLAGS="-DCMAKE_INSTALL_PREFIX=/opt/nvim -DCMAKE_EXE_LINKER_FLAGS='-static -Wl,--export-dynamic'" \
     -j$(nproc)
make install
strip /opt/nvim/bin/nvim
tar czf - -C /opt/nvim . >&3  # write tarball to original stdout
