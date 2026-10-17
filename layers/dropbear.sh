#!/bin/sh
# Builds dropbearmulti: static musl, x86_64, by the pinned zig.
# configure runs on the host shell, sed, and awk (rung 2, like the kernel).
# make is the pinned Alpine make. Password and PAM logins are compiled out.
#
#   SRC        dropbear source tree
#   ZIG        pinned zig
#   HOST_ROOT  pinned Alpine musl tree with make
#   OPTIONS    localoptions.h
#   OUT        dropbearmulti
set -eu

abs() { case $1 in /*) echo "$1" ;; *) echo "$PWD/$1" ;; esac; }
SRC=$(abs "$SRC"); ZIG=$(abs "$ZIG"); HOST_ROOT=$(abs "$HOST_ROOT")
OPTIONS=$(abs "$OPTIONS"); OUT=$(abs "$OUT")

W=$(mktemp -d "${TMPDIR:-/tmp}/dropbear.XXXXXX")
trap 'rm -rf "$W"' EXIT
mkdir -p "$W/bin" "$W/z"
cp -R "$SRC/." "$W/src"
chmod -R u+w "$W/src"
cp "$OPTIONS" "$W/src/localoptions.h"

cat > "$W/bin/make" <<WRAP
#!/bin/sh
exec "$HOST_ROOT/lib/ld-musl-x86_64.so.1" --library-path "$HOST_ROOT/usr/lib:$HOST_ROOT/lib" --argv0 make "$HOST_ROOT/usr/bin/make" "\$@"
WRAP
cat > "$W/bin/cc" <<WRAP
#!/bin/sh
exec "$ZIG" cc -target x86_64-linux-musl -ffile-prefix-map=$W/src=. "\$@"
WRAP
chmod 755 "$W/bin/make" "$W/bin/cc"

export PATH="$W/bin:$PATH" ZIG_GLOBAL_CACHE_DIR="$W/z" ZIG_LOCAL_CACHE_DIR="$W/z"
export LC_ALL=C SOURCE_DATE_EPOCH=0
cd "$W/src"
CC=cc CFLAGS="-Os -ffunction-sections -fdata-sections" \
LDFLAGS="-static -s -Wl,--gc-sections -Wl,--build-id=none" \
./configure --host=x86_64-linux-musl --enable-static --disable-zlib \
    --disable-lastlog --disable-utmp --disable-utmpx --disable-wtmp \
    --disable-wtmpx --disable-pututline --disable-pututxline > configure.out || { cat configure.out >&2; exit 1; }
make -s -j"$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)" \
    PROGRAMS="dropbear dbclient dropbearkey scp" MULTI=1 > make.out 2>&1 ||
    { cat make.out >&2; exit 1; }
cp dropbearmulti "$OUT"
chmod 755 "$OUT"
