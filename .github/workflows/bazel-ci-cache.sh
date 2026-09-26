#!/usr/bin/env bash
# Point Bazel repository_cache and disk_cache at directories actions/cache saves.
# Invoked only from ci.yml. Local builds do not run this and have no .bazelrc.ci.
set -euo pipefail

mode="${1:-}"
repo_unix="${HOME}/.cache/devlayer/bazel-repository"
disk_unix="${HOME}/.cache/devlayer/bazel-disk"
bzlisk_unix="${HOME}/.cache/devlayer/bazelisk"
# Keep each job's disk cache well under the 10GB actions/cache entry cap.
disk_limit=$((512 * 1024 * 1024))

case "$mode" in
configure)
  mkdir -p "$repo_unix" "$disk_unix" "$bzlisk_unix"
  if [ "${RUNNER_OS:-}" = "Windows" ]; then
    repo="$(cygpath -m "$repo_unix")"
    disk="$(cygpath -m "$disk_unix")"
    bzlisk="$(cygpath -m "$bzlisk_unix")"
    # Short output root. Long runfiles break when 8.3 names are off.
    # https://bazel.build/configure/windows
    mkdir -p /c/tmp
  else
    repo="$repo_unix"
    disk="$disk_unix"
    bzlisk="$bzlisk_unix"
  fi
  {
    printf 'build --repository_cache=%s\n' "$repo"
    # Default repo contents cache is {repository_cache}/contents and has no
    # size cap (age GC only, and only after the server idles). Leave it out
    # of the uploaded tree.
    printf 'build --repo_contents_cache=\n'
    printf 'build --disk_cache=%s\n' "$disk"
    if [ "${RUNNER_OS:-}" = "Windows" ]; then
      printf 'startup --output_user_root=C:/tmp\n'
    fi
  } > "${GITHUB_WORKSPACE}/.bazelrc.ci"
  # BAZELISK_HOME is the cache directory itself, not its parent.
  printf 'BAZELISK_HOME=%s\n' "$bzlisk" >> "$GITHUB_ENV"
  ;;
prune)
  # Called after `bazel shutdown`. Do not invoke bazel from this script:
  # on Windows the caller may be Git Bash, which rewrites // labels.
  #
  # Do not use --experimental_disk_cache_gc_* here. In 9.0.1 that GC runs
  # only after the server has been idle (default 5m), which is after this
  # job would have saved the cache, and a concurrent GC can fail the next
  # invocation (fixed in 9.3.0).
  [ -d "$disk_unix" ] || exit 0
  if command -v python3 >/dev/null 2>&1; then
    py=python3
  elif command -v python >/dev/null 2>&1; then
    py=python
  else
    echo "no python to cap the bazel disk cache" >&2
    exit 1
  fi
  DISK="$disk_unix" LIMIT="$disk_limit" "$py" - <<'PY'
import os, stat, sys
root = os.environ["DISK"]
limit = int(os.environ["LIMIT"])
files = []
total = 0
for dp, _, fns in os.walk(root):
    for fn in fns:
        p = os.path.join(dp, fn)
        try:
            st = os.lstat(p)
        except OSError:
            continue
        if not stat.S_ISREG(st.st_mode):
            continue
        files.append((st.st_mtime, st.st_size, p))
        total += st.st_size
if total <= limit:
    print("bazel disk cache %d bytes" % total)
    sys.exit(0)
files.sort()
removed = 0
for _, size, p in files:
    if total <= limit:
        break
    try:
        os.remove(p)
    except OSError:
        continue
    total -= size
    removed += 1
print("pruned %d bazel disk cache files, %d bytes left" % (removed, total))
PY
  ;;
*)
  echo "usage: bazel-ci-cache.sh configure|prune" >&2
  exit 2
  ;;
esac
