#!/bin/sh
# GNU make built with zig. Apple make is not used.
set -eu
out=$1
zig=$2
sdk_tar=$3
configure=$4
toolbox=$5
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
configure=$(abs "$configure")
box=$(abs "$toolbox")
out=$(abs "$out")

case $(uname -s)-$(uname -m) in
  Darwin-arm64) host=aarch64-macos ;;
  Darwin-x86_64) host=x86_64-macos ;;
  Linux-x86_64) host=x86_64-linux-musl ;;
  Linux-aarch64 | Linux-arm64) host=aarch64-linux-musl ;;
  *)
    echo "host make does not support $(uname -s)-$(uname -m)" >&2
    exit 1
    ;;
esac

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
# configure calls ls -t. The toolbox ls accepts -t.
# make, install, strip, and xcrun fail.
good=$work/good
deny=$work/deny
mkdir -p "$good" "$deny" "$work/z" "$(dirname "$out")"
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
export ZIG_LOCAL_CACHE_DIR="$work/z"
export ZIG_GLOBAL_CACHE_DIR="$work/z"
export HOME="$work/z"

cc="$zig cc -target $host -fno-sanitize=undefined"
ldflags=
if [ "$(uname -s)" = Darwin ]; then
  tar -xf "$sdk_tar" -C "$work"
  sdk=$work/MacOSX.sdk
  if [ ! -d "$sdk/usr/include" ]; then
    echo "macos sdk archive has no headers" >&2
    exit 1
  fi
  cc="$cc --sysroot $sdk -mmacosx-version-min=11.0"
  ldflags="-Wl,-dead_strip_dylibs"
else
  ldflags="-static"
fi

cp -R "$(dirname "$configure")/." "$work/src"
cd "$work/src"
export CC="$cc"
export AR="$zig ar"
export RANLIB="$zig ranlib"
export LDFLAGS="$ldflags"
# Configure asks for ld by name. zig cc is that linker.
cat > "$good/ld" << EOF
#!/bin/sh
if [ "\$1" = -v ] || [ "\$1" = -V ] || [ "\$1" = --version ]; then
  echo zig ld
  exit 0
fi
exec $cc "\$@"
EOF
chmod 755 "$good/ld"
export LD="$good/ld"
./configure --disable-nls --disable-dependency-tracking
./build.sh
cp make "$out"
chmod 755 "$out"
