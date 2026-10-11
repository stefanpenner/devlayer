/*
---
why: `_start` calls `tiny`.

uses: show.sh

how: show.sh links this file three times.
Each link passes `-ltiny`.

The dyn-x86-64 link passes `-fno-pie` and `-no-pie`.
The dyn-x86-64 loader is /lib/ld-musl-x86_64.so.1.
The dyn-x86-64 header records libtiny.so.
The kind of dyn-x86-64 is dynamic.

The dyn-pie-x86-64 link passes `-fPIE` and `-pie`.
The dyn-pie-x86-64 loader is /lib/ld-musl-x86_64.so.1.
The dyn-pie-x86-64 header records libtiny.so.
The kind of dyn-pie-x86-64 is dynamic-pie.

The dyn-aarch64 link passes `-fPIE`.
The dyn-aarch64 link entry is `__start`.
The dyn-aarch64 loader is /usr/lib/dyld.
The dyn-aarch64 header records libtiny.dylib.
The dyn-aarch64 header records /usr/lib/libSystem.B.dylib.
The kind of dyn-aarch64 is dynamic-pie.
---
*/

void tiny(void);

void _start(void) {
    tiny();
}
