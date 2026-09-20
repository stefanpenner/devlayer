#!/bin/sh
# Bazel --workspace_status_command. Must be an already-runnable executable
# (not a built target) because Bazel invokes it before the compile.
echo "STABLE_VERSION $(git describe --tags --always --dirty 2>/dev/null || echo dev)"
