#!/bin/sh
# ---
# why: This file links one Darwin cargo binary with the pinned SDK.
# uses: bazel
# how: The cargo genrule calls this file as the linker.
# ---
# Do not keep -L. A search path can record a host install name.
# -liconv becomes the static archive. The dylib is a host library.
# libSystem stays a -l flag. A .tbd path is a second load.
set -eu
fw="$SDK/System/Library/Frameworks"
dirs=""
pendir=0
for arg in "$@"; do
  if [ "$pendir" = 1 ]; then
    pendir=0
    dirs="$dirs
$arg"
    continue
  fi
  case "$arg" in
    -L) pendir=1 ;;
    -L*)
      dirs="$dirs
${arg#-L}"
      ;;
  esac
done
n=$#
i=0
pendir=0
pendlib=0
while [ "$i" -lt "$n" ]; do
  arg=$1
  shift
  i=$((i + 1))
  if [ "$pendir" = 1 ]; then
    pendir=0
    continue
  fi
  if [ "$pendlib" = 1 ]; then
    pendlib=0
    arg="-l$arg"
  fi
  case "$arg" in
    -Wl,-l*|-Wl,-L*)
      printf 'refusing %s\n' "$arg" >&2
      exit 1
      ;;
    -L) pendir=1 ;;
    -L*) ;;
    -l) pendlib=1 ;;
    -liconv)
      if [ -f "$ICONV" ]; then
        set -- "$@" "$ICONV"
      else
        printf 'refusing %s\n' "$arg" >&2
        exit 1
      fi
      ;;
    -lSystem|-lc|-lm|-ldl|-lpthread|-lc++|-lc++abi|-lobjc)
      set -- "$@" "$arg"
      ;;
    -l*)
      name=${arg#-l}
      found=""
      oldifs=$IFS
      IFS='
'
      set -f
      for d in $dirs; do
        [ -n "$d" ] || continue
        case "$d" in
          /opt/homebrew|/opt/homebrew/*|/usr/local|/usr/local/*) continue ;;
        esac
        if [ -f "$d/lib$name.a" ]; then
          found="$d/lib$name.a"
          break
        fi
      done
      set +f
      IFS=$oldifs
      if [ -n "$found" ]; then
        set -- "$@" "$found"
      else
        printf 'refusing %s\n' "$arg" >&2
        exit 1
      fi
      ;;
    /usr/lib/libSystem.B.dylib|/usr/lib/libobjc.A.dylib|/usr/lib/libc++.1.dylib|/usr/lib/libc++abi.dylib|/usr/lib/libutil.dylib|/usr/lib/libresolv.9.dylib)
      set -- "$@" "$arg"
      ;;
    /opt/homebrew/*|/usr/local/*)
      case "$arg" in
        *.dylib|*.tbd|*.so)
          printf 'refusing %s\n' "$arg" >&2
          exit 1
          ;;
      esac
      set -- "$@" "$arg"
      ;;
    /usr/lib/*)
      printf 'refusing %s\n' "$arg" >&2
      exit 1
      ;;
    *) set -- "$@" "$arg" ;;
  esac
done
if [ "$pendlib" = 1 ]; then
  printf 'refusing -l\n' >&2
  exit 1
fi
exec "$ZIG" cc -target "$TARGET" --sysroot "$SDK" -isystem "$SDK/usr/include" \
  -iframework "$fw" -F "$fw" -mmacosx-version-min=11.0 "$@"
