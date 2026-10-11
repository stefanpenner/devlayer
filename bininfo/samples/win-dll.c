/*
---
why: tiny.dll exports `tiny`.

uses: show.sh

how: The function `tiny` has an empty body.
This file defines `_DllMainCRTStartup`.
show.sh links this file as tiny.dll.
show.sh passes `/implib` for tiny.lib.
tiny.dll is a PE shared library.
---
*/

void tiny(void) {}

void _DllMainCRTStartup(void) {}
