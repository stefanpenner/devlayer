#!/bin/sh
# Apple ranlib drops ELF objects from a zig ar archive.
set -eu
exec "$NVIM_ZIG" ranlib "$@"
