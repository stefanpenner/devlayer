#!/bin/sh
# Run one static tool from a scratch image.
# Usage: smoke.sh ARCHIVE PLATFORM TOOL
# PLATFORM is linux/amd64 or linux/arm64.
# The tool tarball is extracted, then copied into FROM scratch.
# Docker still mounts /proc and /sys. Nothing else is in the image.
set -eu

archive=$1
platform=$2
tool=$3

root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
tar -xzf "$archive" -C "$root"

# entry: program path inside the image
# env:   optional Dockerfile ENV line
# expect: grep -q pattern on stdout
case "$tool" in
btop)
	entry=/btop
	env=
	set -- --version
	expect=btop
	test -x "$root/btop"
	;;
git)
	entry=/bin/git
	env="ENV GIT_EXEC_PATH=/libexec/git-core"
	set -- --version
	expect='^git version'
	test -x "$root/bin/git"
	test -d "$root/libexec/git-core"
	;;
zsh)
	entry=/bin/zsh
	fns=$(find "$root/share/zsh" -type d -name functions | head -n 1)
	test -n "$fns"
	env="ENV FPATH=/${fns#"$root"/}"
	set -- -fc 'print smoke-ok'
	expect='^smoke-ok$'
	test -x "$root/bin/zsh"
	;;
nvim)
	entry=/bin/nvim
	env="ENV VIMRUNTIME=/share/nvim/runtime"
	set -- --headless --version
	expect='^NVIM v'
	test -x "$root/bin/nvim"
	test -d "$root/share/nvim/runtime"
	;;
*)
	echo "unknown tool $tool" >&2
	exit 2
	;;
esac

cat > "$root/Dockerfile" <<EOF
FROM scratch
COPY . /
$env
ENTRYPOINT ["$entry"]
EOF

image="devlayer-smoke-$tool-$$"
docker build --platform "$platform" -t "$image" "$root" >/dev/null
out=$(docker run --rm --network=none --platform "$platform" "$image" "$@")
printf '%s\n' "$out" | grep -q "$expect"
