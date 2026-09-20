#!/bin/sh
# Build git as a static binary with HTTPS support.
# Expects GIT_VERSION env var.
set -euo pipefail

exec 3>&1  # save stdout for final tar output
exec 1>&2  # redirect all build output to stderr

curl -fsSL "https://github.com/git/git/archive/refs/tags/v${GIT_VERSION}.tar.gz" | tar xz
cd git-${GIT_VERSION}

cat > config.mak << 'EOF'
prefix = /opt/git
NO_TCLTK = YesPlease
NO_GETTEXT = YesPlease
NO_PERL = YesPlease
NO_PYTHON = YesPlease
NO_EXPAT = YesPlease
NO_NSEC = YesPlease
NO_REGEX = YesPlease
NO_RUST = YesPlease
CFLAGS = -Os -DNDEBUG
LDFLAGS = -static -Wl,--allow-multiple-definition
EOF
echo "CURL_LDFLAGS = $(pkg-config --static --libs libcurl | sed 's/-ldl//g')" >> config.mak

make -j$(nproc)
make install
strip /opt/git/bin/git
find /opt/git/libexec -type f -name 'git*' | while read f; do
  file "$f" | grep -q ELF && strip "$f" || true
done
tar czf - -C /opt/git . >&3  # write tarball to original stdout
