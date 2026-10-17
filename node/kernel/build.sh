#!/bin/sh
# Builds the node kernel: allnoconfig plus one fragment, LLVM=1, no modules.
#
# Inputs (absolute or relative to $PWD):
#   KERNEL_TAR   pinned linux-<v>.tar.xz
#   LLVM_BIN     pinned kernel.org LLVM bin directory
#   HOST_ROOT    pinned Alpine musl tree: flex, bison, m4, bc, xz, zstd, make, libelf.a
#   ZIG          pinned zig. It is HOSTCC: static musl host programs.
#   FRAGMENT     the config fragment
#   OUT_BZIMAGE  OUT_CONFIG  OUT_CPIO_TOOL
#   NPROC        optional job count
set -eu

abs() { case "$1" in /*) printf '%s\n' "$1" ;; *) printf '%s/%s\n' "$PWD" "$1" ;; esac; }
KERNEL_TAR=$(abs "$KERNEL_TAR"); LLVM_BIN=$(abs "$LLVM_BIN"); HOST_ROOT=$(abs "$HOST_ROOT")
ZIG=$(abs "$ZIG"); FRAGMENT=$(abs "$FRAGMENT")
OUT_BZIMAGE=$(abs "$OUT_BZIMAGE"); OUT_CONFIG=$(abs "$OUT_CONFIG"); OUT_CPIO_TOOL=$(abs "$OUT_CPIO_TOOL")

case "$(uname -m)" in
x86_64) MUSL=x86_64 ZT=x86_64-linux-musl ;;
aarch64 | arm64) MUSL=aarch64 ZT=aarch64-linux-musl ;;
*) echo "build.sh: unsupported exec cpu $(uname -m)" >&2; exit 1 ;;
esac

W=$(abs "${TMPDIR:-.}/kbuild.$$")
rm -rf "$W"; mkdir -p "$W/bin" "$W/zig-cache"
trap 'rm -rf "$W"' EXIT

# Pinned musl tools. The loader comes from the same packages, not the host.
for t in flex bison m4 bc xz zstd make; do
    cat > "$W/bin/$t" <<WRAP
#!/bin/sh
exec "$HOST_ROOT/lib/ld-musl-$MUSL.so.1" --library-path "$HOST_ROOT/usr/lib:$HOST_ROOT/lib" --argv0 "$W/bin/$t" "$HOST_ROOT/usr/bin/$t" "\$@"
WRAP
    chmod 755 "$W/bin/$t"
done
# libelf.a carries its own crc32.o. libz.a defines the same CRC-32. zig's lld
# has no muldefs flag, so the copy drops the libelf member and keeps zlib's.
mkdir -p "$W/lib"
cp "$HOST_ROOT/usr/lib/libelf.a" "$W/lib/libelf.a"
chmod 644 "$W/lib/libelf.a"
"$LLVM_BIN/llvm-ar" d "$W/lib/libelf.a" crc32.o

# HOSTCC is zig. Host programs are static musl, linked with the pinned libelf.a.
cat > "$W/bin/hostcc" <<WRAP
#!/bin/sh
exec "$ZIG" cc -target $ZT -static -isystem "$HOST_ROOT/usr/include" -L"$W/lib" -L"$HOST_ROOT/usr/lib" "\$@"
WRAP
chmod 755 "$W/bin/hostcc"

export PATH="$W/bin:$LLVM_BIN:$PATH"
export M4="$W/bin/m4" BISON_PKGDATADIR="$HOST_ROOT/usr/share/bison"
export ZIG_GLOBAL_CACHE_DIR="$W/zig-cache" ZIG_LOCAL_CACHE_DIR="$W/zig-cache"
export LC_ALL=C TZ=UTC
# Reproducible build identity.
export SOURCE_DATE_EPOCH=0
export KBUILD_BUILD_TIMESTAMP="1970-01-01 00:00:00 UTC"
export KBUILD_BUILD_USER=devlayer KBUILD_BUILD_HOST=devlayer KBUILD_BUILD_VERSION=1

xz -dc "$KERNEL_TAR" | tar -x -C "$W"
SRC=$(echo "$W"/linux-*)
O="$W/out"
J=${NPROC:-$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)}

# The tree path must not reach the image. Kbuild maps srctree; map the object tree too.
KCFLAGS="-mtune=znver4 -ffile-prefix-map=$W/="
mk() {
    make -C "$SRC" O="$O" -j"$J" ARCH=x86_64 LLVM=1 HOSTCC=hostcc HOSTCXX=hostcc \
        HOSTPKG_CONFIG=false "HOSTLDFLAGS=-lz -lzstd" "$@"
}

# allnoconfig, then the fragment. -n means allnoconfig. merge_config reports
# each requested value that did not reach .config; any such line fails the build.
mkdir -p "$O"
(cd "$SRC" && ARCH=x86_64 LLVM=1 HOSTCC=hostcc HOSTPKG_CONFIG=false \
    sh scripts/kconfig/merge_config.sh -n -O "$O" "$FRAGMENT") > "$W/merge.log" 2>&1 || {
    cat "$W/merge.log" >&2; exit 1; }
if grep 'Value requested' "$W/merge.log" >/dev/null; then
    grep -A2 'Value requested' "$W/merge.log" >&2 || true
    echo "build.sh: the fragment did not reach .config" >&2
    exit 1
fi

mk KCFLAGS="$KCFLAGS" bzImage >&2

cp "$O/arch/x86/boot/bzImage" "$OUT_BZIMAGE"
cp "$O/.config" "$OUT_CONFIG"
cp "$O/usr/gen_init_cpio" "$OUT_CPIO_TOOL"
