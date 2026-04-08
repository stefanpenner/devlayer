# Multi-stage build: compile static binaries + download pre-built tools
# Stages run in parallel via BuildKit for faster builds

FROM alpine:3.21 AS base

RUN apk add --no-cache \
    build-base cmake git curl wget linux-headers \
    ncurses-dev ncurses-static \
    go ninja gettext-tiny-dev unzip autoconf automake libtool pkgconf \
    zlib-dev zlib-static \
    openssl-dev openssl-libs-static \
    curl-dev curl-static \
    expat-dev expat-static \
    pcre2-dev perl \
    nghttp2-static brotli-static \
    libidn2-static libunistring-static libpsl-static zstd-static

WORKDIR /build

# ============================================================
# git — static with HTTPS support
# ============================================================
FROM base AS git-build
ARG GIT_VERSION=2.53.0
RUN curl -fsSL "https://github.com/git/git/archive/refs/tags/v${GIT_VERSION}.tar.gz" | tar xz && \
    cd git-${GIT_VERSION} && \
    echo "prefix = /opt/git" > config.mak && \
    echo "NO_TCLTK = YesPlease" >> config.mak && \
    echo "NO_GETTEXT = YesPlease" >> config.mak && \
    echo "NO_PERL = YesPlease" >> config.mak && \
    echo "NO_PYTHON = YesPlease" >> config.mak && \
    echo "NO_EXPAT = YesPlease" >> config.mak && \
    echo "NO_NSEC = YesPlease" >> config.mak && \
    echo "NO_REGEX = YesPlease" >> config.mak && \
    echo "CURL_LDFLAGS = $(pkg-config --static --libs libcurl | sed 's/-ldl//g')" >> config.mak && \
    echo "CFLAGS = -Os -DNDEBUG" >> config.mak && \
    echo "LDFLAGS = -static -Wl,--allow-multiple-definition" >> config.mak && \
    make -j$(nproc) && \
    make install && \
    strip /opt/git/bin/git && \
    find /opt/git/libexec -type f -name 'git*' | while read f; do file "$f" | grep -q ELF && strip "$f" || true; done

# ============================================================
# zsh — static with essential modules
# ============================================================
FROM base AS zsh-build
# Use latest master — zsh-5.9 has termcap conflicts with newer ncurses
RUN git clone --depth 1 https://github.com/zsh-users/zsh.git && \
    cd zsh && \
    ./Util/preconfig && \
    ./configure \
      --prefix=/opt/zsh \
      --enable-static \
      --disable-dynamic \
      --enable-multibyte \
      --with-tcsetpgrp \
      LDFLAGS="-static" \
      CFLAGS="-Os -DNDEBUG" && \
    for mod in compctl complete complist computil zle zutil parameter terminfo datetime stat system mathfunc "net/socket" "net/tcp"; do \
      escaped=$(echo "$mod" | sed 's|/|\\/|g'); \
      sed -i "/name=zsh\/${escaped}/s/link=no/link=static/; /name=zsh\/${escaped}/s/load=no/load=yes/" config.modules; \
    done && \
    make -j$(nproc) && \
    make install.bin install.fns && \
    strip /opt/zsh/bin/zsh

# ============================================================
# htop — static
# ============================================================
FROM base AS htop-build
ARG HTOP_VERSION=3.4.1
RUN git clone --depth 1 --branch ${HTOP_VERSION} https://github.com/htop-dev/htop.git && \
    cd htop && \
    ./autogen.sh && \
    ./configure --enable-static LDFLAGS="-static" CFLAGS="-Os -DNDEBUG" && \
    make -j$(nproc) && \
    strip htop

# ============================================================
# btop — static
# ============================================================
FROM base AS btop-build
ARG BTOP_VERSION=1.4.6
RUN curl -fsSL "https://github.com/aristocratos/btop/archive/refs/tags/v${BTOP_VERSION}.tar.gz" | tar xz && \
    cd btop-${BTOP_VERSION} && \
    cmake -B build \
      -DCMAKE_BUILD_TYPE=Release \
      -DBTOP_STATIC=ON \
      -DBTOP_GPU=OFF \
      -DBTOP_LTO=ON && \
    cmake --build build -j$(nproc) && \
    find build -name btop -type f | head -1 | xargs -I{} cp {} /usr/local/bin/btop && \
    strip /usr/local/bin/btop

# ============================================================
# neovim — static
# ============================================================
FROM base AS nvim-build
ARG NVIM_VERSION=0.12.0
RUN git clone --depth 1 --branch v${NVIM_VERSION} https://github.com/neovim/neovim.git && \
    cd neovim && \
    make CMAKE_BUILD_TYPE=Release \
         CMAKE_EXTRA_FLAGS="-DCMAKE_INSTALL_PREFIX=/opt/nvim -DCMAKE_EXE_LINKER_FLAGS='-static -Wl,--export-dynamic'" \
         -j$(nproc) && \
    make install && \
    strip /opt/nvim/bin/nvim

# ============================================================
# GNU make — static
# ============================================================
FROM base AS make-build
ARG MAKE_VERSION=4.4.1
RUN curl -fsSL "https://ftp.gnu.org/gnu/make/make-${MAKE_VERSION}.tar.gz" | tar xz && \
    cd make-${MAKE_VERSION} && \
    ./configure CFLAGS="-Os -DNDEBUG" LDFLAGS="-static" && \
    make -j$(nproc) && \
    strip make

# ============================================================
# Download pre-built binaries + assemble tarball
# ============================================================
FROM ubuntu:24.04 AS assembler

RUN apt-get update && apt-get install -y --no-install-recommends \
      curl ca-certificates unzip file xz-utils \
    && rm -rf /var/lib/apt/lists/*

COPY versions.env /tmp/versions.env
COPY scripts/download-binaries.sh /tmp/scripts/download-binaries.sh

# Download pre-built binaries (skip nvim — we built it from source)
RUN SKIP_NVIM=1 bash /tmp/scripts/download-binaries.sh /staging linux

# Add compiled static binaries
COPY --from=git-build /opt/git /staging/git/
COPY --from=zsh-build /opt/zsh /staging/zsh/
COPY --from=htop-build /build/htop/htop /staging/bin/htop
COPY --from=btop-build /usr/local/bin/btop /staging/bin/btop
COPY --from=nvim-build /opt/nvim /staging/nvim/
COPY --from=make-build /build/make-*/make /staging/bin/make

# Create wrapper scripts in bin/ (set env vars so tools are self-contained)
RUN printf '#!/bin/sh\nPREFIX="$(cd "$(dirname "$0")/.." && pwd)"\nexport GIT_EXEC_PATH="$PREFIX/git/libexec/git-core"\nexec "$PREFIX/git/bin/git" "$@"\n' > /staging/bin/git && \
    printf '#!/bin/sh\nPREFIX="$(cd "$(dirname "$0")/.." && pwd)"\nfor d in "$PREFIX"/zsh/share/zsh/*/functions; do [ -d "$d" ] && export FPATH="$d${FPATH:+:$FPATH}" && break; done\nexec "$PREFIX/zsh/bin/zsh" "$@"\n' > /staging/bin/zsh && \
    printf '#!/bin/sh\nPREFIX="$(cd "$(dirname "$0")/.." && pwd)"\nexport VIMRUNTIME="$PREFIX/nvim/share/nvim/runtime"\nexec "$PREFIX/nvim/bin/nvim" "$@"\n' > /staging/bin/nvim && \
    printf '#!/bin/sh\nPREFIX="$(cd "$(dirname "$0")/.." && pwd)"\nexport GOROOT="$PREFIX/go"\nexec "$PREFIX/go/bin/go" "$@"\n' > /staging/bin/go && \
    printf '#!/bin/sh\nPREFIX="$(cd "$(dirname "$0")/.." && pwd)"\nexport GOROOT="$PREFIX/go"\nexec "$PREFIX/go/bin/gofmt" "$@"\n' > /staging/bin/gofmt && \
    printf '#!/bin/sh\nPREFIX="$(cd "$(dirname "$0")/.." && pwd)"\nexec "$PREFIX/zig/zig" "$@"\n' > /staging/bin/zig && \
    printf '#!/bin/sh\nPREFIX="$(cd "$(dirname "$0")/.." && pwd)"\nexec "$PREFIX/zig/zig" cc "$@"\n' > /staging/bin/cc && \
    printf '#!/bin/sh\nPREFIX="$(cd "$(dirname "$0")/.." && pwd)"\nexec "$PREFIX/zig/zig" c++ "$@"\n' > /staging/bin/c++ && \
    chmod +x /staging/bin/*

# Verify compiled binaries are static
RUN echo "==> Verifying static linkage:" && \
    for f in /staging/git/bin/git /staging/zsh/bin/zsh /staging/bin/htop /staging/bin/btop /staging/nvim/bin/nvim /staging/bin/make; do \
      echo "  $(basename $f): $(file $f | grep -o 'statically linked' || echo 'dynamically linked')"; \
    done

# Verify wrapper scripts are correct
RUN echo "==> Verifying wrappers:" && \
    for f in git zsh nvim go gofmt zig cc c++; do \
      head -1 /staging/bin/$f | grep -q '#!/bin/sh' || { echo "FAIL: bin/$f is not a wrapper script"; exit 1; }; \
      echo "  bin/$f: wrapper ok"; \
    done

# Generate checksums
RUN find /staging -type f -executable | sort | xargs sha256sum > /staging/SHA256SUMS

# Package
RUN tar czf /devlayer.tar.gz -C /staging .

CMD ["cat", "/devlayer.tar.gz"]
