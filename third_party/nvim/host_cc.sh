#!/bin/sh
# Host C compiler for LuaJIT minilua and buildvm.
# Darwin links with zig and the pinned SDK.
set -eu
if [ -z "${NVIM_ZIG:-}" ]; then
  echo "NVIM_ZIG is empty" >&2
  exit 1
fi
if [ -z "${ZIG_LOCAL_CACHE_DIR:-}" ]; then
  echo "ZIG_LOCAL_CACHE_DIR is empty" >&2
  exit 1
fi
mkdir -p "$ZIG_LOCAL_CACHE_DIR" "${ZIG_GLOBAL_CACHE_DIR:-$ZIG_LOCAL_CACHE_DIR}"
case "$(uname -s)-$(uname -m)" in
  Darwin-arm64) target=aarch64-macos ;;
  Darwin-x86_64) target=x86_64-macos ;;
  Linux-x86_64) target=x86_64-linux-gnu ;;
  Linux-aarch64 | Linux-arm64) target=aarch64-linux-gnu ;;
  *)
    echo "host cc does not support $(uname -s)-$(uname -m)" >&2
    exit 1
    ;;
esac
set -- -fno-sanitize=undefined "$@"
if [ "$(uname -s)" = Darwin ]; then
  if [ -z "${DEVLAYER_SDKROOT:-}" ]; then
    echo "DEVLAYER_SDKROOT is empty" >&2
    exit 1
  fi
  # cmake passes the pin tar. One extract serves every HOSTCC job.
  if [ ! -d "$DEVLAYER_SDKROOT" ]; then
    dest="$ZIG_LOCAL_CACHE_DIR/MacOSX.sdk"
    lock="$ZIG_LOCAL_CACHE_DIR/sdk.extract.lock"
    if [ ! -d "$dest/usr/include" ]; then
      if mkdir "$lock" 2>/dev/null; then
        stage="$ZIG_LOCAL_CACHE_DIR/sdk.partial"
        rm -rf "$stage"
        mkdir -p "$stage"
        if ! tar -xf "$DEVLAYER_SDKROOT" -C "$stage"; then
          rm -rf "$stage"
          rmdir "$lock"
          echo "macos sdk extract failed" >&2
          exit 1
        fi
        if [ ! -d "$stage/MacOSX.sdk/usr/include" ]; then
          rm -rf "$stage"
          rmdir "$lock"
          echo "macos sdk archive has no headers" >&2
          exit 1
        fi
        mv "$stage/MacOSX.sdk" "$dest"
        rm -rf "$stage"
        rmdir "$lock"
      else
        while [ ! -d "$dest/usr/include" ]; do
          if [ ! -d "$lock" ]; then
            echo "macos sdk extract failed" >&2
            exit 1
          fi
          sleep 0.2
        done
      fi
    fi
    DEVLAYER_SDKROOT=$dest
  fi
  export DEVELOPER_DIR=/tmp/devlayer-no-xcrun
  exec "$NVIM_ZIG" cc -target "$target" --sysroot "$DEVLAYER_SDKROOT" -mmacosx-version-min=11.0 "$@"
fi
exec "$NVIM_ZIG" cc -target "$target" "$@"
