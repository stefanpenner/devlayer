/*
---
why: libtiny.so exports `tiny` from this file.
libtiny.dylib exports `tiny` from this file.

uses: show.sh

how: show.sh links this file as libtiny.so.
show.sh passes `-soname` `libtiny.so`.
libtiny.so is an ELF shared library.
The libtiny.so header records no library.

show.sh links this file as libtiny.dylib.
show.sh passes `-install_name` `libtiny.dylib`.
libtiny.dylib is a Mach-O shared library.
The libtiny.dylib target is aarch64-macos.
The libtiny.dylib header records the loader /usr/lib/dyld.
The libtiny.dylib header records the library /usr/lib/libSystem.B.dylib.
---
*/

void tiny(void) {}
