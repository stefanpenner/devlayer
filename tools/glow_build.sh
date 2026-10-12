#!/bin/sh
# Compile one glow binary from a vendored module tree.
# CGO is off. The module mode is vendor. The network is off.
set -eu
go_bin=$1
src=$2
out=$3
goos=$4
goarch=$5
version_file=$6
version=$(tr -d '\n' < "$version_file")
goroot=$(CDPATH= cd -- "$(dirname "$go_bin")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/devlayer-glow.XXXXXX")
trap 'rm -rf "$work"' EXIT
export GOROOT="$goroot"
export GOOS="$goos"
export GOARCH="$goarch"
export CGO_ENABLED=0
export GOPROXY=off
export GOSUMDB=off
export GOFLAGS=-mod=vendor
export GOTOOLCHAIN=local
export GO111MODULE=on
export GOCACHE="$work/cache"
export GOTMPDIR="$work"
export HOME="$work"
export GOPATH="$work/gopath"
export PATH="$goroot/bin:/usr/bin:/bin"
# Bazel inputs are symlinks. go:embed rejects a symlink.
mkdir -p "$work/src"
cp -R -L "$src/." "$work/src/"
cd "$work/src"
"$go_bin" build -trimpath -tags netgo -ldflags="-s -w -X main.Version=${version}" -o "$out" .
