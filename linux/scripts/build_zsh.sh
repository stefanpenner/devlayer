#!/bin/sh
# Build zsh as a static binary with essential modules.
# Expects ZSH_VERSION env var (zsh-users tag is zsh-$ZSH_VERSION).
set -euo pipefail

exec 3>&1  # save stdout for final tar output
exec 1>&2  # redirect all build output to stderr

git clone --depth 1 --branch "zsh-${ZSH_VERSION}" https://github.com/zsh-users/zsh.git
cd zsh
./Util/preconfig
./configure \
  --prefix=/opt/zsh \
  --enable-static \
  --disable-dynamic \
  --enable-multibyte \
  --with-tcsetpgrp \
  LDFLAGS="-static" \
  CFLAGS="-Os -DNDEBUG"

for mod in compctl complete complist computil zle zutil parameter terminfo datetime stat system mathfunc "net/socket" "net/tcp"; do
  escaped=$(echo "$mod" | sed 's|/|\\/|g')
  sed -i "/name=zsh\/$escaped/s/link=no/link=static/; /name=zsh\/$escaped/s/load=no/load=yes/" config.modules
done

make -j$(nproc)
make install.bin install.fns
strip /opt/zsh/bin/zsh
tar czf - -C /opt/zsh . >&3  # write tarball to original stdout
