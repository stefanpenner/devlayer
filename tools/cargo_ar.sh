#!/bin/sh
# ---
# why: This file archives C objects for one cargo crate.
# uses: bazel
# how: The cargo genrule sets AR to this file.
# ---
set -eu
if [ -z "${ZIG:-}" ]; then
  printf 'cargo_ar: ZIG is empty\n' >&2
  exit 1
fi
exec "$ZIG" ar "$@"
