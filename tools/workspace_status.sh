#!/bin/sh
# Bazel --workspace_status_command (Unix). Already runnable; not a built target.
# No --dirty: that refreshes the index. On Windows the same refresh never
# returns (run 35561178234, BazelWorkspaceStatusAction, critical path 1004s).
ver=$(git --no-optional-locks -c core.fsmonitor=false describe --tags --always 2>/dev/null) || ver=dev
[ -n "$ver" ] || ver=dev
echo "STABLE_VERSION $ver"
