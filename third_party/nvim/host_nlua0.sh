#!/bin/sh
# Host generator module. The target LuaJIT is static and cannot dlopen this file.
set -eu
out=$1
zig=$2
luajit=$3
lpeg_tar=$4
host_cc=$5
nlua=$6
toolbox=$7
root=$(pwd)
export NVIM_ZIG="$root/$zig"
box="$root/$toolbox"
nlua="$root/$nlua"
cache=$(mktemp -d)
work=$(mktemp -d)
trap 'rm -rf "$cache" "$work"' EXIT
mkdir -p "$cache"
export ZIG_LOCAL_CACHE_DIR="$cache"
export ZIG_GLOBAL_CACHE_DIR="$cache"
export HOME="$cache"
deny=$cache/deny
mkdir -p "$deny"
for tool in make install strip xcrun; do
  cat > "$deny/$tool" << EOF
#!/bin/sh
echo "hermetic violation: $tool" >&2
exit 1
EOF
  chmod 755 "$deny/$tool"
done
export PATH="$box:$deny"
export DEVELOPER_DIR=/tmp/devlayer-no-xcrun
if [ "$(uname -s)" = Darwin ]; then
  if [ -z "${DEVLAYER_SDKROOT:-}" ]; then
    echo "DEVLAYER_SDKROOT is empty" >&2
    exit 1
  fi
  case $DEVLAYER_SDKROOT in
    /*) sdk_tar=$DEVLAYER_SDKROOT ;;
    *) sdk_tar=$root/$DEVLAYER_SDKROOT ;;
  esac
  tar -xf "$sdk_tar" -C "$work"
  if [ ! -d "$work/MacOSX.sdk/usr/include" ]; then
    echo "macos sdk archive has no headers" >&2
    exit 1
  fi
  export DEVLAYER_SDKROOT=$work/MacOSX.sdk
fi
nvim=$(dirname "$(dirname "$nlua")")
tar -xzf "$lpeg_tar" -C "$work"
lpeg=$(find "$work" -mindepth 1 -maxdepth 1 -type d | head -1)
extra=
case $(uname -s) in
  Darwin) extra="-undefined dynamic_lookup -Wl,-install_name,@rpath/host_nlua0.so" ;;
  Linux) extra="-Wl,--allow-shlib-undefined" ;;
  *)
    echo "host nlua0 supports Darwin and Linux" >&2
    exit 1
    ;;
esac
# shellcheck disable=SC2086
"$root/$host_cc" -shared -fPIC -std=c99 -DNVIM_NLUA0 \
  -I "$root/$luajit/include/luajit-2.1" \
  -I "$nvim/src" \
  -I "$lpeg" \
  $extra \
  -o "$root/$out" \
  "$nvim/src/nlua0.c" \
  "$nvim/src/mpack/conv.c" \
  "$nvim/src/mpack/lmpack.c" \
  "$nvim/src/mpack/mpack_core.c" \
  "$nvim/src/mpack/object.c" \
  "$nvim/src/mpack/rpc.c" \
  "$lpeg/lpcap.c" \
  "$lpeg/lpcode.c" \
  "$lpeg/lpcset.c" \
  "$lpeg/lpprint.c" \
  "$lpeg/lptree.c" \
  "$lpeg/lpvm.c"
chmod 755 "$root/$out"
