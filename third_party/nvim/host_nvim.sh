#!/bin/sh
# helptags runs on the host. The Linux nvim cannot run here.
set -eu
case "$(uname -s)" in
  Darwin) bin=$NVIM_HOST_DARWIN ;;
  Linux)
    case "$(uname -m)" in
      x86_64 | amd64) bin=$NVIM_HOST_LINUX ;;
      aarch64 | arm64) bin=$NVIM_HOST_LINUX_ARM64 ;;
      *)
        printf 'no host nvim for Linux %s\n' "$(uname -m)" >&2
        exit 1
        ;;
    esac
    ;;
  *)
    printf 'no host nvim for %s\n' "$(uname -s)" >&2
    exit 1
    ;;
esac
root=$(CDPATH= cd -- "$(dirname "$bin")/.." && pwd)
export VIMRUNTIME="$root/share/nvim/runtime"
exec "$bin" "$@"
