/*
---
why: `_start` imports `tiny` from tiny.dll.

uses: show.sh

how: The `tiny` declaration is `__declspec(dllimport)`.
show.sh links this file.
show.sh passes `-ltiny`.

The link output equals dyn-pie-x86-64.exe.
The dyn-pie-x86-64.exe header records tiny.dll.
Zig enables DYNAMIC_BASE on dyn-pie-x86-64.exe.
The kind of dyn-pie-x86-64.exe is dynamic-pie.

show.sh copies dyn-pie-x86-64.exe to dyn-x86-64.exe.
show.sh clears DYNAMIC_BASE on dyn-x86-64.exe.
The kind of dyn-x86-64.exe is dynamic.
---
*/

__declspec(dllimport) void tiny(void);

void _start(void) { tiny(); }
