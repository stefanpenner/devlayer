#!/bin/sh
# Build one static archive. The compiler is zig cc. Apple ar is not used.
set -eu

root=$(pwd)
deny=$(mktemp -d)
trap 'rm -rf "$deny"' EXIT
mkdir -p "$deny"
for tool in make install strip xcrun; do
  cat > "$deny/$tool" << EOF
#!/bin/sh
echo "hermetic violation: $tool" >&2
exit 1
EOF
  chmod 755 "$deny/$tool"
done

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

zig=$(resolve "$(abs "$1")")
sdk_tar=$(abs "$2")
out=$(abs "$3")
recipe=$(abs "$4")
box=$(abs "$5")
export PATH="$box:$deny"
export DEVELOPER_DIR=/tmp/devlayer-no-xcrun

cache=$(mktemp -d)
work=$(mktemp -d)
trap 'rm -rf "$cache" "$work" "$deny"' EXIT
mkdir -p "$cache" "$work/src" "$work/obj"
tar -xf "$sdk_tar" -C "$work"
sysroot=$work/MacOSX.sdk
if [ ! -d "$sysroot/usr/include" ]; then
  echo "macos sdk archive has no headers" >&2
  exit 1
fi
export ZIG_LOCAL_CACHE_DIR="$cache"
export ZIG_GLOBAL_CACHE_DIR="$cache"
: > "$work/args"
: > "$work/units"
target=

add() {
  printf '%s\0' "$1" >> "$work/args"
}

extract() {
  name=$1
  tarfile=$2
  dest=$work/src/$name
  mkdir -p "$dest"
  tar -xzf "$tarfile" -C "$dest" --strip-components=1
}

while IFS=$(printf '\t') read -r key a b; do
  case $key in
    "") continue ;;
    target) target=$a ;;
    tar) extract "$a" "$(abs "$b")" ;;
    patch) /usr/bin/patch -p1 -d "$work/src/$a" < "$(abs "$b")" ;;
    inc) add "-I$work/src/$a/$b" ;;
    def) add "-D$a" ;;
    flag) add "$a" ;;
    unit) printf '%s\t%s\n' "$a" "$b" >> "$work/units" ;;
    *)
      echo "bad recipe line: $key" >&2
      exit 1
      ;;
  esac
done < "$recipe"

if [ -z "${target}" ]; then
  echo "recipe has no target" >&2
  exit 1
fi

n=0
while IFS=$(printf '\t') read -r alias rel; do
  if [ -z "$alias" ]; then
    continue
  fi
  n=$((n + 1))
  src=$work/src/$alias/$rel
  if [ ! -f "$src" ]; then
    echo "missing source $alias/$rel" >&2
    exit 1
  fi
  base=$(basename "$rel" .c).o
  obj=$work/obj/$base
  if [ -e "$obj" ]; then
    echo "duplicate object $base" >&2
    exit 1
  fi
  {
    printf '%s\0' "$zig" cc -target "$target" --sysroot "$sysroot"
    cat "$work/args"
    printf '%s\0' -c -o "$obj" "$src"
  } > "$work/cmd"
  # The compiler line is one argv. /bin/sh is the script interpreter.
  xargs -0 /bin/sh -c 'exec "$@"' sh < "$work/cmd"
done < "$work/units"

if [ "$n" -eq 0 ]; then
  echo "recipe has no sources" >&2
  exit 1
fi

mkdir -p "$(dirname "$out")"
rm -f "$out"
"$zig" ar rc "$out" "$work/obj"/*.o
"$zig" ranlib "$out"
