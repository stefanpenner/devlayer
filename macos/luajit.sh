#!/bin/sh
# Static Mach-O LuaJIT.
# Host tools run on this machine. Objects use the target triple.
set -eu
out=$1
zig=$2
sdk_tar=$3
src_tar=$4
target=$5
make=$6
toolbox=$7
root=$(pwd)

abs() {
  case $1 in
    /*) printf '%s\n' "$1" ;;
    *) printf '%s\n' "$root/$1" ;;
  esac
}

resolve() {
  f=$1
  while [ -L "$f" ]; do
    link=$(readlink "$f")
    case $link in
      /*) f=$link ;;
      *) f=$(dirname "$f")/$link ;;
    esac
  done
  dir=$(cd "$(dirname "$f")" && pwd)
  printf '%s\n' "$dir/$(basename "$f")"
}

zig=$(resolve "$(abs "$zig")")
sdk_tar=$(abs "$sdk_tar")
src_tar=$(abs "$src_tar")
target_cc_triple=$target
make=$(abs "$make")
box=$(abs "$toolbox")
out=$(abs "$out")

case $(uname -s)-$(uname -m) in
  Darwin-arm64) host=aarch64-macos ;;
  Darwin-x86_64) host=x86_64-macos ;;
  Linux-x86_64) host=x86_64-linux-musl ;;
  Linux-aarch64 | Linux-arm64) host=aarch64-linux-musl ;;
  *)
    echo "host cc does not support $(uname -s)-$(uname -m)" >&2
    exit 1
    ;;
esac

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
# make, install, strip, and xcrun fail. ls and tar come from the toolbox.
deny=$work/deny
mkdir -p "$work/z" "$work/bin" "$deny" "$(dirname "$out")"
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
export ZIG_LOCAL_CACHE_DIR="$work/z"
export ZIG_GLOBAL_CACHE_DIR="$work/z"
export HOME="$work/z"

# The pin is a ustar archive. GNU tar rejects -z on that file.
tar -xf "$sdk_tar" -C "$work"
sdk=$work/MacOSX.sdk
if [ ! -d "$sdk/usr/include" ]; then
  echo "macos sdk archive has no headers" >&2
  exit 1
fi

tar -xzf "$src_tar" -C "$work"
src=$(find "$work" -mindepth 1 -maxdepth 1 -type d -name 'LuaJIT-*' | head -1)
if [ -z "$src" ]; then
  echo "luajit source is missing" >&2
  exit 1
fi

host_flags="-fno-sanitize=undefined"
if [ "$(uname -s)" = Darwin ]; then
  host_flags="$host_flags --sysroot $sdk -mmacosx-version-min=11.0"
fi
cat > "$work/bin/hostcc" << EOF
#!/bin/sh
exec "$zig" cc -target $host $host_flags "\$@"
EOF
cat > "$work/bin/targetcc" << EOF
#!/bin/sh
exec "$zig" cc -target $target_cc_triple --sysroot "$sdk" -mmacosx-version-min=11.0 -fno-sanitize=undefined "\$@"
EOF
printf '%s\n' '#!/bin/sh' 'exit 0' > "$work/bin/strip"
chmod 755 "$work/bin/hostcc" "$work/bin/targetcc" "$work/bin/strip"
export PATH="$work/bin:$box:$deny"
export MACOSX_DEPLOYMENT_TARGET=11.0

jobs=$(nproc)
"$make" -C "$src/src" -j"$jobs" \
  HOST_CC="$work/bin/hostcc" \
  CC="$work/bin/targetcc" \
  TARGET_SYS=Darwin \
  BUILDMODE=static \
  TARGET_AR="$zig ar rcus" \
  libluajit.a
cp "$src/src/libluajit.a" "$out"
