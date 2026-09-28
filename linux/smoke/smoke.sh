#!/bin/sh
# Run one static tool from a scratch image.
# Usage: smoke.sh ARCHIVE PLATFORM ENTRY ENV EXPECT -- ARG...
# ENV is empty, KEY=VALUE, or FPATH=@functions.
# @functions is the share/zsh/*/functions directory inside the archive.
# Docker still mounts /proc and /sys. Nothing else is in the image.
set -eu

archive=$1
platform=$2
entry=$3
envspec=$4
expect=$5
shift 5
test "$1" = "--"
shift

root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
tar -xzf "$archive" -C "$root"
test -x "$root$entry"

envline=
if [ -n "$envspec" ]; then
	key=${envspec%%=*}
	val=${envspec#*=}
	if [ "$val" = "@functions" ]; then
		fns=$(find "$root/share/zsh" -type d -name functions | head -n 1)
		test -n "$fns"
		val=/${fns#"$root"/}
	fi
	test -e "$root$val"
	envline="ENV $key=$val"
fi

cat > "$root/Dockerfile" <<EOF
FROM scratch
COPY . /
$envline
ENTRYPOINT ["$entry"]
EOF

image="devlayer-smoke-$$"
docker build --platform "$platform" -t "$image" "$root" >/dev/null
out=$(docker run --rm --network=none --platform "$platform" "$image" "$@")
printf '%s\n' "$out" | grep -q "$expect"
