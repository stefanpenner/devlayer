/*
---
why: This `_start` calls no function.

uses: show.sh

how: show.sh assigns this file to src.
Each zig cc command reads src.
Each command has its own flags.

rel-x86-64.o is an ELF object.
The kind of rel-x86-64.o is rel.

exec-x86-64 is an ELF program.
The kind of exec-x86-64 is static.
The exec-x86-64 header records no loader.
The exec-x86-64 header records no library.

static-pie-x86-64 is an ELF program.
The kind of static-pie-x86-64 is static-pie.
The static-pie-x86-64 header records no loader.
The static-pie-x86-64 header records no library.

lib-x86-64.so is an ELF shared library.
The lib-x86-64.so header records no library.

show.sh repeats the exec-x86-64 link flags for exec-i386, exec-aarch64, and exec-powerpc64.
The kind of exec-i386, exec-aarch64, and exec-powerpc64 is static.

The exec-i386 target is x86-linux-musl.
The exec-i386 machine is i386.

The exec-aarch64 target is aarch64-linux-musl.
The exec-aarch64 machine is AArch64.

The exec-powerpc64 target is powerpc64-linux-musl.
The exec-powerpc64 byte order is big-endian.

rel-aarch64.o is a Mach-O object.
The kind of rel-aarch64.o is rel.

dyn-pie-aarch64 is a Mach-O program.
The kind of dyn-pie-aarch64 is dynamic-pie.
The dyn-pie-aarch64 target is aarch64-macos-none.
The dyn-pie-aarch64 link entry is `__start`.
The dyn-pie-aarch64 header records the loader /usr/lib/dyld.
The dyn-pie-aarch64 header records the library /usr/lib/libSystem.B.dylib.

show.sh copies dyn-pie-aarch64 to dyn-fixed-aarch64.
show.sh clears the MH_PIE bit on dyn-fixed-aarch64.
The kind of dyn-fixed-aarch64 is dynamic.

rel-x86-64.obj is a COFF object.
The kind of rel-x86-64.obj is rel.

exec-x86-64.exe is a PE32+ program.
The kind of exec-x86-64.exe is static-pie.
Zig enables DYNAMIC_BASE on exec-x86-64.exe.

exec-i386.exe is a PE32 program.
The kind of exec-i386.exe is static-pie.
Zig enables DYNAMIC_BASE on exec-i386.exe.

show.sh copies exec-x86-64.exe to static-x86-64.exe.
show.sh clears DYNAMIC_BASE on static-x86-64.exe.
The kind of static-x86-64.exe is static.
---
*/

void _start(void) {}
