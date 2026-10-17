#!/bin/sh
# Builds LibreSSL's libcrypto, then OpenSSH portable against it: static musl,
# x86_64, by the pinned zig. Both configure scripts run on the host shell,
# sed, and awk (rung 2, like the kernel). make is the pinned Alpine make.
#
#   OPENSSH_SRC  LIBRESSL_SRC  source trees
#   ZIG          pinned zig
#   HOST_ROOT    pinned Alpine musl tree with make
#   OUT_DIR      gets sshd sshd-session sshd-auth ssh ssh-keygen
set -eu

abs() { case $1 in /*) echo "$1" ;; *) echo "$PWD/$1" ;; esac; }
OPENSSH_SRC=$(abs "$OPENSSH_SRC"); LIBRESSL_SRC=$(abs "$LIBRESSL_SRC")
ZIG=$(abs "$ZIG"); HOST_ROOT=$(abs "$HOST_ROOT"); OUT_DIR=$(abs "$OUT_DIR")

W=$(mktemp -d "${TMPDIR:-/tmp}/openssh.XXXXXX")
trap 'rm -rf "$W"' EXIT
mkdir -p "$W/bin" "$W/z" "$W/crypto"
cp -R "$LIBRESSL_SRC/." "$W/libressl"
cp -R "$OPENSSH_SRC/." "$W/openssh"
chmod -R u+w "$W/libressl" "$W/openssh"

cat > "$W/bin/make" <<WRAP
#!/bin/sh
exec "$HOST_ROOT/lib/ld-musl-x86_64.so.1" --library-path "$HOST_ROOT/usr/lib:$HOST_ROOT/lib" --argv0 make "$HOST_ROOT/usr/bin/make" "\$@"
WRAP
cat > "$W/bin/cc" <<WRAP
#!/bin/sh
exec "$ZIG" cc -target x86_64-linux-musl -ffile-prefix-map=$W=. "\$@"
WRAP
for t in ar ranlib; do
    printf '#!/bin/sh\nexec "%s" %s "$@"\n' "$ZIG" "$t" > "$W/bin/$t"
done
chmod 755 "$W/bin/"*

export PATH="$W/bin:$PATH" ZIG_GLOBAL_CACHE_DIR="$W/z" ZIG_LOCAL_CACHE_DIR="$W/z"
export LC_ALL=C SOURCE_DATE_EPOCH=0 CC=cc AR=ar RANLIB=ranlib
J=$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)

log() { "$@" > "$W/log" 2>&1 || { cat "$W/log" >&2; exit 1; }; }

cd "$W/libressl"
# --with-openssldir: the default bakes $W/crypto/etc/ssl into libcrypto, and
# $W is a random temp dir. Nothing on the node reads /etc/ssl.
log env CFLAGS="-Os -ffunction-sections -fdata-sections" ./configure \
    --host=x86_64-linux-musl --disable-shared --enable-static --disable-tests \
    --prefix="$W/crypto" --with-openssldir=/etc/ssl
log make -j"$J" -C crypto install
log make -C include install

cd "$W/openssh"
# --without-pie: libcrypto.a is not PIC, and a static binary gains nothing.
# PKCS#11 stays off: the host key is served by node-identity over
# HostKeyAgent, and sshd never loads a PKCS#11 provider for host keys.
log env CFLAGS="-Os -ffunction-sections -fdata-sections" \
    LDFLAGS="-static -s -Wl,--gc-sections -Wl,--build-id=none" \
    ./configure --host=x86_64-linux-musl --without-pie \
    --with-ssl-dir="$W/crypto" --without-zlib --without-shadow \
    --disable-pkcs11 --disable-security-key \
    --disable-lastlog --disable-utmp --disable-utmpx --disable-wtmp \
    --disable-wtmpx --disable-pututline --disable-pututxline \
    --prefix=/ --bindir=/bin --sbindir=/sbin --libexecdir=/libexec \
    --sysconfdir=/etc/ssh --with-pid-dir=/run \
    --with-privsep-path=/var/empty --with-privsep-user=sshd \
    --with-default-path=/bin:/sbin --with-superuser-path=/bin:/sbin
log make -j"$J" sshd sshd-session sshd-auth ssh ssh-keygen
mkdir -p "$OUT_DIR"
for b in sshd sshd-session sshd-auth ssh ssh-keygen; do
    cp "$b" "$OUT_DIR/$b"
    chmod 755 "$OUT_DIR/$b"
done
