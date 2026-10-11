#!/bin/sh
# Build a host LuaJIT that can dlopen host-nlua0. Zig picks the exec machine.
set -eu
out=$1
zig=$2
tar=$3
host_cc=$4
make=$5
toolbox=$6
root=$(pwd)
zig="$root/$zig"
host_cc="$root/$host_cc"
make="$root/$make"
box="$root/$toolbox"
export NVIM_ZIG="$zig"
cache=$(mktemp -d)
work=$(mktemp -d)
trap 'rm -rf "$cache" "$work"' EXIT
mkdir -p "$cache"
export ZIG_LOCAL_CACHE_DIR="$cache"
export ZIG_GLOBAL_CACHE_DIR="$cache"
export HOME="$cache"
prefix="$root/$out"
good=$cache/good
deny=$cache/deny
mkdir -p "$good" "$deny"
cat > "$good/install" << 'EOF'
#!/bin/sh
set -eu
mode=755
dirs=0
while [ $# -gt 0 ]; do
  case $1 in
    -m) mode=$2; shift 2 ;;
    -d) dirs=1; shift ;;
    -c|-C|-p|-s|-v) shift ;;
    --) shift; break ;;
    -*) echo "install: bad flag $1" >&2; exit 1 ;;
    *) break ;;
  esac
done
if [ "$dirs" -eq 1 ]; then
  mkdir -p "$@"
  exit 0
fi
if [ $# -lt 2 ]; then
  echo "install: need source and dest" >&2
  exit 1
fi
n=$#
dest=
for dest in "$@"; do
  :
done
i=0
for src in "$@"; do
  i=$((i + 1))
  if [ "$i" -eq "$n" ]; then
    break
  fi
  if [ -d "$dest" ]; then
    cp "$src" "$dest/$(basename "$src")"
    chmod "$mode" "$dest/$(basename "$src")"
  else
    mkdir -p "$(dirname "$dest")"
    cp "$src" "$dest"
    chmod "$mode" "$dest"
  fi
done
EOF
printf '%s\n' '#!/bin/sh' 'exit 0' > "$good/strip"
chmod 755 "$good/install" "$good/strip"
for tool in make install strip xcrun; do
  cat > "$deny/$tool" << EOF
#!/bin/sh
echo "hermetic violation: $tool" >&2
exit 1
EOF
  chmod 755 "$deny/$tool"
done
export PATH="$good:$box:$deny"
export DEVELOPER_DIR=/tmp/devlayer-no-xcrun
tar -xzf "$tar" -C "$work"
src=$(find "$work" -mindepth 1 -maxdepth 1 -type d | head -1)
jobs=$(nproc)
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
  export MACOSX_DEPLOYMENT_TARGET=11.0
fi
case $(uname -s) in
  Darwin)
    "$make" -C "$src" -j"$jobs" \
      CC="$host_cc" HOST_CC="$host_cc" BUILDMODE=dynamic PREFIX="$prefix" \
      TARGET_DYLIBPATH='@rpath/$(TARGET_DYLIBNAME)' \
      LDFLAGS='-Wl,-rpath,@loader_path/../lib' \
      install
    ;;
  Linux)
    "$make" -C "$src" -j"$jobs" \
      CC="$host_cc" HOST_CC="$host_cc" BUILDMODE=dynamic PREFIX="$prefix" \
      LDFLAGS='-Wl,-rpath,$ORIGIN/../lib -lunwind' install
    ;;
  *)
    echo "host luajit supports Darwin and Linux" >&2
    exit 1
    ;;
esac
chmod 755 "$prefix/bin/luajit"
