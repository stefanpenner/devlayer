#!/bin/sh
# Exec toolbox. This action may call /bin/bash.
# macOS sed has no T command, so a small toybox sed runs scripts/make.sh.
# The product scripts do not put /bin or /usr/bin on PATH.
set -eu

out=$1
zig=$2
sdk_tar=$3
anchor=$4
mini=$5
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
anchor=$(abs "$anchor")
mini=$(abs "$mini")
out=$(abs "$out")

case $(uname -s)-$(uname -m) in
  Darwin-arm64) host=aarch64-macos ;;
  Darwin-x86_64) host=x86_64-macos ;;
  Linux-x86_64) host=x86_64-linux-musl ;;
  Linux-aarch64 | Linux-arm64) host=aarch64-linux-musl ;;
  *)
    echo "toolbox does not support $(uname -s)-$(uname -m)" >&2
    exit 1
    ;;
esac

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/z" "$work/prereq" "$work/obj" "$work/src" "$out"
export DEVELOPER_DIR=/tmp/devlayer-no-xcrun
export ZIG_LOCAL_CACHE_DIR="$work/z"
export ZIG_GLOBAL_CACHE_DIR="$work/z"
export HOME="$work/z"
export LANG=C
export LC_ALL=C

cp -R "$(dirname "$anchor")/." "$work/src"
cd "$work/src"

ldflags=
if [ "$(uname -s)" = Darwin ]; then
  ldflags="-Wl,-dead_strip_dylibs"
  # Bootstrap tar. The product scripts use the toolbox tar.
  tar -xf "$sdk_tar" -C "$work"
  sdk=$work/MacOSX.sdk
  if [ ! -d "$sdk/usr/include" ]; then
    echo "macos sdk archive has no headers" >&2
    exit 1
  fi
  cat > "$work/cc" << EOF
#!/bin/sh
exec "$zig" cc -target $host -fno-sanitize=undefined --sysroot "$sdk" -isystem "$sdk/usr/include" -mmacosx-version-min=11.0 "\$@"
EOF
else
  ldflags="-static"
  cat > "$work/cc" << EOF
#!/bin/sh
exec "$zig" cc -target $host -fno-sanitize=undefined "\$@"
EOF
fi
chmod 755 "$work/cc"

# scripts/prereq/build.sh names the commands whose headers already exist.
# Those headers do not declare tar, awk, find, or cp.
files=
state=0
while IFS= read -r line || [ -n "$line" ]; do
  if [ "$state" -eq 0 ]; then
    case $line in
      FILES=*) state=1 ;;
    esac
    continue
  fi
  if [ "$line" = '"' ]; then
    break
  fi
  files="$files $line"
  state=2
done < scripts/prereq/build.sh
if [ "$state" -ne 2 ]; then
  echo "prereq file list is missing" >&2
  exit 1
fi

cflags="-funsigned-char -I scripts/prereq -I . -Os -ffunction-sections -fdata-sections -fno-asynchronous-unwind-tables -fno-strict-aliasing -Wno-deprecated-declarations -Wno-string-plus-int -Wno-invalid-source-encoding"
objs=
set -- lib/*.c
for word in $files; do
  set -- "$@" "$word"
done
for f in "$@"; do
  o=$work/obj/$(printf '%s' "$f" | tr '/' '_').o
  "$work/cc" $cflags -c "$f" -o "$o"
  objs="$objs $o"
done
# shellcheck disable=SC2086
"$work/cc" $objs $ldflags -o "$work/prereq/toybox"

# Link only real prereq names. A cp symlink would hide /bin/cp.
"$work/prereq/toybox" | while read -r line; do
  for name in $line; do
    ln -s "$work/prereq/toybox" "$work/prereq/$name"
  done
done

export PATH="$work/prereq:/bin:/usr/bin:/usr/sbin"
export SED="$work/prereq/sed"
export CC="$work/cc"
export HOSTCC="$work/cc"
export LDFLAGS="$ldflags"
export NOSTRIP=1
export STRIP=true
KCONFIG_ALLCONFIG="$mini" /bin/bash scripts/genconfig.sh -n
/bin/bash scripts/make.sh

if [ ! -x toybox ]; then
  echo "toybox binary is missing" >&2
  exit 1
fi
cp toybox "$out/toybox"
chmod 755 "$out/toybox"
./toybox | while read -r line; do
  for name in $line; do
    case $name in
      toybox|install|strip|make|xcrun) continue ;;
    esac
    ln -s toybox "$out/$name"
  done
done
