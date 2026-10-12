#!/bin/sh
# ---
# why: This file compiles C for one cargo crate.
# uses: bazel
# how: The cargo genrule sets CC to this file. cc-rs adds a host triple. This file drops that triple.
# ---
# cc-plain already has the pinned zig target and the pinned sysroot.
set -eu
if [ -z "${CC_PLAIN:-}" ]; then
  printf 'cargo_cc: CC_PLAIN is empty\n' >&2
  exit 1
fi
skip=0
n=0
for arg in "$@"; do
  if [ "$skip" = 1 ]; then
    skip=0
    continue
  fi
  case "$arg" in
    --target|-target|--sysroot|-isysroot|-arch|-mmacosx-version-min|-iframework|-F)
      skip=1
      ;;
    --target=*|-target=*|--sysroot=*|-isysroot=*|-arch=*|-mmacosx-version-min=*|-iframework=*|-F*)
      ;;
    *)
      n=$((n + 1))
      eval "keep$n=\$arg"
      ;;
  esac
done
set --
i=1
while [ "$i" -le "$n" ]; do
  eval "set -- \"\$@\" \"\$keep$i\""
  i=$((i + 1))
done
exec "$CC_PLAIN" "$@"
