#!/bin/sh
# Smoke-test one static tool in a scratch container.
# The image has no shell and no libraries. Docker's default /proc and /sys
# mounts are the only kernel interfaces. Everything else is the tool tree.
set -eu

arch=$1
archive=$2
kind=$3

case "$arch" in
x86_64) platform=linux/amd64 ;;
aarch64) platform=linux/arm64 ;;
*) echo "unknown arch $arch" >&2; exit 2 ;;
esac

root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
tar -xzf "$archive" -C "$root"
image="devlayer-smoke-$kind-$$"

# scratch is not a runnable image name. Build a one-off image that contains
# only the files copied in the Dockerfile below, then run that.
build() {
	docker build --platform "$platform" -t "$image" "$root" >/dev/null
}
run() {
	docker run --rm --network=none --platform "$platform" "$image" "$@"
}

case "$kind" in
btop)
	test -x "$root/btop"
	cat > "$root/Dockerfile" <<'EOF'
FROM scratch
COPY btop /btop
ENTRYPOINT ["/btop"]
EOF
	build
	out=$(run --version)
	printf '%s\n' "$out" | grep -q btop
	;;
git)
	test -x "$root/bin/git"
	test -d "$root/libexec/git-core"
	cat > "$root/Dockerfile" <<'EOF'
FROM scratch
COPY . /
ENV GIT_EXEC_PATH=/libexec/git-core
ENTRYPOINT ["/bin/git"]
EOF
	build
	out=$(run --version)
	printf '%s\n' "$out" | grep -q '^git version'
	;;
zsh)
	test -x "$root/bin/zsh"
	fns=$(find "$root/share/zsh" -type d -name functions | head -n 1)
	test -n "$fns"
	rel=${fns#"$root"/}
	cat > "$root/Dockerfile" <<EOF
FROM scratch
COPY . /
ENV FPATH=/$rel
ENTRYPOINT ["/bin/zsh"]
EOF
	build
	# print is a builtin. A missing libc fails here. A missing functions
	# directory already failed the test above.
	out=$(run -fc 'print smoke-ok')
	printf '%s\n' "$out" | grep -q '^smoke-ok$'
	;;
nvim)
	test -x "$root/bin/nvim"
	test -d "$root/share/nvim/runtime"
	cat > "$root/Dockerfile" <<'EOF'
FROM scratch
COPY . /
ENV VIMRUNTIME=/share/nvim/runtime
ENTRYPOINT ["/bin/nvim"]
EOF
	build
	out=$(run --headless --version)
	printf '%s\n' "$out" | grep -q '^NVIM v'
	;;
*)
	echo "unknown kind $kind" >&2
	exit 2
	;;
esac
