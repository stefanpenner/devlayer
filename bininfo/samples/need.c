/*
---
why: The libneed.so header records DT_NEEDED libtiny.so.

uses: show.sh

how: This file calls `tiny`.
This file does not define `tiny`.

show.sh links this file.
show.sh passes `-ltiny`.
The link output equals libneed.so.
show.sh passes `-soname` `libneed.so`.
---
*/

void tiny(void);

void need(void) {
    tiny();
}
