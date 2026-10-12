<!--
---
why: This file tells a reader what bininfo prints and how to run it.
uses: a reader
how: The reader opens this file in the repo.
---
-->

# bininfo

## What is this?

`bininfo` reads one ELF, Mach-O, or PE file.
It also reads one zip, one dmg, or one tiff file.
An ARW file is a tiff file whose make is SONY.
It prints one page for that file.
ELF, Mach-O, and PE share one page.
That page has type, load, symbols, sizes, where, and sections.
A zip, a dmg, and a tiff each print their own page.

## Why does it exist?

ELF, Mach-O, and PE use different words for the same start fact.
`bininfo` uses one word for that fact on each format.
One claim can check that fact on each format.
A failed claim exits 1.
A zip, a dmg, and a tiff have no start claim.

## Other tools

Each tool below does one part of that job.

| tool | formats | job | missing |
| --- | --- | --- | --- |
| [`llvm-readobj`](https://llvm.org/docs/CommandGuide/llvm-readobj.html) | ELF, Mach-O, COFF | Raw headers and dynamic tags. | The shared start words. A claim file. |
| [`rabin2`](https://book.rada.re/tools/rabin2/intro.html) | Many formats. | Interpreter, libraries, and flags. | The shared start words. A claim file. |
| [`knife`](https://github.com/bl4ckr0ss3/knife) | ELF, Mach-O, PE | Imports, hashes, and disassembly. | The teaching page. A claim check. |
| [`eu-elfclassify`](https://man.archlinux.org/man/eu-elfclassify.1.en) | ELF | Exit 0 when the file matches a type flag. | Mach-O and PE. |
| [`annocheck`](https://sourceware.org/annobin/annobin.html/Annocheck.html) | ELF | Exit on a hardening profile. | Mach-O, PE, and the link list. |
| [`checksec`](https://github.com/slimm609/checksec) | ELF | Hardening flags. | Mach-O and PE. |
| [`readelf`](https://sourceware.org/binutils/docs/binutils/readelf.html) `-d` | ELF | `NEEDED` and the interpreter. | Mach-O and PE. |
| [`ldd`](https://man.archlinux.org/man/ldd.1.en) | ELF | Libraries the loader would open. | The header page. It can run the loader. |
| [`otool`](https://keith.github.io/xcode-man-pages/otool.1.html) `-L` | Mach-O | Linked libraries. | ELF and PE. |
| [`dumpbin`](https://learn.microsoft.com/en-us/cpp/build/reference/dumpbin-reference) `/imports` | PE | Imported DLLs. | ELF and Mach-O. |

[`llvm-readobj`](https://llvm.org/docs/CommandGuide/llvm-readobj.html) is the closest reader.
[`eu-elfclassify`](https://man.archlinux.org/man/eu-elfclassify.1.en) is the closest check.

## Install the command

This crate lives in the devlayer repo at `bininfo/`.
Cargo writes the `bininfo` command.

```
cargo install --path .
```

The crate is not on crates.io. `publish` is false.

## Use the library

The same repo is a library. The command calls that library.
`elf`, `macho`, and `pe` each read one start format.
`zip`, `dmg`, and `tiff` each read one container.
`osload` checks host libraries on a start file.
`claim` checks one start claim.
`open` reads any of these formats.
`facts` returns an error for a zip, a dmg, or a tiff.

A Rust project can depend on a local checkout.

```
bininfo = { path = "../bininfo" }
```

```rust
let bytes = std::fs::read(path)?;
let file = bininfo::open(&bytes, false)?;
let page = bininfo::render(path, &file, false, false);
let facts = bininfo::facts(path, &file, &bininfo::current_host())?;
let claims = bininfo::claim::parse(&["hermetic".to_string()])?;
let misses = bininfo::claim::judge(&facts, &claims);
```

`rows` on `open` keeps symbol rows. `json` on `render` prints one JSON object.

## How to use the command

Build with Cargo.
Run the tool from this directory.

```
cargo run --quiet -- <file>
```

Put each flag before the file name.

```
cargo run --quiet -- [--json] [--symbols] [--check] [--expect <claim>] <file>
cargo run --quiet -- [--json] --claims <file>
cargo run --quiet -- --policy <directory> <path>
```

A common `needed` name has a trailing note.
The note names what that library provides.
An unknown name has no note.
`delay` uses the same notes.

`--json` prints one JSON object.
`--symbols` prints each symbol row.
`--check` checks the `hermetic` claim.
`--expect` checks one claim.
Repeat `--expect` for each claim.
`--claims` checks each line in a claim file.
Add `--json` to print a check as JSON.

`hermetic` passes when all three facts are true.

- `kind` is `static` or `static-pie`.
- `interp` is null.
- `needed` is empty.

This check does not read `dlopen`.

`--policy` checks one file or a directory.
The policy root is a directory.
One TOML file is one binary.
The file path, minus `.toml`, is the binary path.
Each key is the record name in that file.
ELF uses `DT_NEEDED` and `PT_INTERP`.
Mach-O uses `LC_LOAD_DYLIB` and `LC_LOAD_DYLINKER`.
PE uses `IMAGE_IMPORT_DESCRIPTOR` and `ImgDelayDescr`.
Each library name has a why.
A bare name is an error.
A record key that is absent means that record is empty.
An absent `flags` key does not check header flags.
An absent `cpusubtype` key does not check the CPU subtype.
An absent dylib version does not check that version.

A script passes.
A binary with no policy file fails.
`policy/bin/devlayer.toml` does not name `libiconv` or `libz`.
A symlink that leaves the directory fails.
This check does not change `hermetic`.

A `kind` value is `static`, `dynamic`, `dynamic-pie`, `static-pie`, `shared`, `rel`, or `other`.
Repeat `needed=` once for each library.
`interp=null` means the file has no loader.

```
cargo run --quiet -- --expect kind=dynamic-pie --expect needed=libtiny.so <file>
```

A terminal adds color to the page.
`NO_COLOR` keeps the page plain.

Build the samples with Zig.

```
./show.sh
./check.sh
```

`./show.sh` prints JSON for each sample.
`./check.sh` checks `claims.txt`.
