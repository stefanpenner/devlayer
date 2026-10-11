#!/bin/sh
set -eu
cd "$(dirname "$0")"
./show.sh >/dev/null
bin=$(cargo build --message-format=json --quiet | sed -n 's/.*"executable":"\([^"]*\)".*/\1/p' | head -n 1)
if [ -z "$bin" ]; then
  echo "cargo did not build bininfo" >&2
  exit 1
fi
exec "$bin" --claims claims.txt
