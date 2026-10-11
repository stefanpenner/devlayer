#!/bin/sh
set -eu

cd "$(dirname "$0")"
out=samples/out
src=samples/start.c
mkdir -p "$out"

put_u16() {
  file=$1
  off=$2
  val=$3
  lo=$((val & 255))
  hi=$(((val >> 8) & 255))
  printf "\\$(printf '%03o' "$lo")\\$(printf '%03o' "$hi")" |
    dd of="$file" bs=1 seek="$off" conv=notrunc 2>/dev/null
}

put_u32() {
  file=$1
  off=$2
  val=$3
  b0=$((val & 255))
  b1=$(((val >> 8) & 255))
  b2=$(((val >> 16) & 255))
  b3=$(((val >> 24) & 255))
  printf "\\$(printf '%03o' "$b0")\\$(printf '%03o' "$b1")\\$(printf '%03o' "$b2")\\$(printf '%03o' "$b3")" |
    dd of="$file" bs=1 seek="$off" conv=notrunc 2>/dev/null
}

# Zig sets DYNAMIC_BASE on a Windows exe. Clear that bit.
clear_dynamic_base() {
  cp "$1" "$2"
  lfanew=$(od -An -t u4 -j 60 -N 4 "$2" | tr -d '[:space:]')
  off=$((lfanew + 94))
  dllc=$(od -An -t u2 -j "$off" -N 2 "$2" | tr -d '[:space:]')
  put_u16 "$2" "$off" $((dllc & ~64))
}

# Zig sets MH_PIE on a macOS program. Clear that bit.
clear_mh_pie() {
  cp "$1" "$2"
  flags=$(od -An -t u4 -j 24 -N 4 "$2" | tr -d '[:space:]')
  put_u32 "$2" 24 $((flags & ~2097152))
}

zig cc -target x86_64-linux-musl -c -g0 -fno-unwind-tables -o "$out/rel-x86-64.o" "$src"
zig cc -target x86_64-linux-musl -s -nostdlib -static -fno-pie -no-pie -Wl,-e,_start -o "$out/exec-x86-64" "$src"
zig cc -target x86_64-linux-musl -s -nostdlib -fPIE -pie -Wl,-e,_start -o "$out/static-pie-x86-64" "$src"
zig cc -target x86_64-linux-musl -s -nostdlib -shared -o "$out/lib-x86-64.so" "$src"
zig cc -target x86-linux-musl -s -nostdlib -static -fno-pie -no-pie -Wl,-e,_start -o "$out/exec-i386" "$src"
zig cc -target aarch64-linux-musl -s -nostdlib -static -fno-pie -no-pie -Wl,-e,_start -o "$out/exec-aarch64" "$src"
zig cc -target powerpc64-linux-musl -s -nostdlib -static -fno-pie -no-pie -Wl,-e,_start -o "$out/exec-powerpc64" "$src"
zig cc -target x86_64-linux-musl -s -nostdlib -shared -Wl,-soname,libtiny.so -o "$out/libtiny.so" samples/tiny.c
zig cc -target x86_64-linux-musl -s -nostdlib -shared -L"$out" -ltiny -Wl,-soname,libneed.so -o "$out/libneed.so" samples/need.c
zig cc -target x86_64-linux-musl -s -nostdlib -fno-pie -no-pie -Wl,-e,_start -Wl,-dynamic-linker,/lib/ld-musl-x86_64.so.1 -L"$out" -ltiny -o "$out/dyn-x86-64" samples/call.c
zig cc -target x86_64-linux-musl -s -nostdlib -fPIE -pie -Wl,-e,_start -Wl,-dynamic-linker,/lib/ld-musl-x86_64.so.1 -L"$out" -ltiny -o "$out/dyn-pie-x86-64" samples/call.c
zig cc -target aarch64-macos -c -g0 -o "$out/rel-aarch64.o" "$src"
zig cc -target aarch64-macos -s -nostdlib -shared -Wl,-install_name,libtiny.dylib -o "$out/libtiny.dylib" samples/tiny.c
zig cc -target aarch64-macos-none -s -nostdlib -fPIE -Wl,-e,__start -o "$out/dyn-pie-aarch64" "$src"
zig cc -target aarch64-macos -s -nostdlib -fPIE -Wl,-e,__start -L"$out" -ltiny -o "$out/dyn-aarch64" samples/call.c
zig cc -target x86_64-windows-gnu -c -o "$out/rel-x86-64.obj" "$src"
zig cc -target x86_64-windows-gnu -s -nostdlib -Wl,-e,_start -o "$out/exec-x86-64.exe" "$src"
zig cc -target x86-windows-gnu -s -nostdlib -Wl,-e,_start -o "$out/exec-i386.exe" "$src"
zig cc -target x86_64-windows-gnu -s -nostdlib -shared -Wl,/implib:"$out/tiny.lib" -o "$out/tiny.dll" samples/win-dll.c
zig cc -target x86_64-windows-gnu -s -nostdlib -Wl,-e,_start -L"$out" -ltiny -o "$out/dyn-pie-x86-64.exe" samples/win-call.c

# Zig cannot link a macOS program that does not name dyld.
# These headers are the two hermetic Mac kinds.
printf '\xcf\xfa\xed\xfe\x0c\x00\x00\x01\x00\x00\x00\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00' >"$out/static-aarch64"
printf '\xcf\xfa\xed\xfe\x0c\x00\x00\x01\x00\x00\x00\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x20\x00\x00\x00\x00\x00' >"$out/static-pie-aarch64"
clear_mh_pie "$out/dyn-pie-aarch64" "$out/dyn-fixed-aarch64"
clear_dynamic_base "$out/exec-x86-64.exe" "$out/static-x86-64.exe"
clear_dynamic_base "$out/dyn-pie-x86-64.exe" "$out/dyn-x86-64.exe"

bin=$(cargo build --message-format=json --quiet | sed -n 's/.*"executable":"\([^"]*\)".*/\1/p' | head -n 1)
if [ -z "$bin" ]; then
  echo "cargo did not build bininfo" >&2
  exit 1
fi

for file in \
  rel-x86-64.o \
  exec-x86-64 \
  static-pie-x86-64 \
  lib-x86-64.so \
  libtiny.so \
  libneed.so \
  dyn-x86-64 \
  dyn-pie-x86-64 \
  exec-i386 \
  exec-aarch64 \
  exec-powerpc64 \
  rel-aarch64.o \
  libtiny.dylib \
  static-aarch64 \
  static-pie-aarch64 \
  dyn-fixed-aarch64 \
  dyn-pie-aarch64 \
  dyn-aarch64 \
  rel-x86-64.obj \
  exec-x86-64.exe \
  static-x86-64.exe \
  exec-i386.exe \
  tiny.dll \
  dyn-x86-64.exe \
  dyn-pie-x86-64.exe
do
  "$bin" --json "$out/$file"
done
