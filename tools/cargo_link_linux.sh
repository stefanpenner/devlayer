#!/bin/sh
# ---
# why: This file links one Linux cargo binary with no dynamic loader.
# uses: bazel
# how: The cargo genrule calls this file as the linker.
# ---
# Drop -no-pie. Zig 0.16 writes static PIE when -static stays.
# A dynamic loader path names another file.
set -eu
n=$#
i=0
pend=0
while [ "$i" -lt "$n" ]; do
  arg=$1
  shift
  i=$((i + 1))
  if [ "$pend" = 1 ]; then
    pend=0
    continue
  fi
  case "$arg" in
    -no-pie|-Bdynamic|-Wl,-Bdynamic) ;;
    -dynamic-linker) pend=1 ;;
    -Wl,-dynamic-linker,*|-Wl,--dynamic-linker,*|-Wl,-dynamic-linker=*|-Wl,--dynamic-linker=*) ;;
    *) set -- "$@" "$arg" ;;
  esac
done
exec "$ZIG" cc -target "$TARGET" -static "$@"
