# Hermeticity

devlayer has two jobs.
The first job is a portable hermetic build.
The second job installs a sysroot.
A build target, a test target, and a run target use that sysroot.

The sysroot is `$DEVLAYER_PREFIX` or `~/.local` on Unix.
The sysroot is `%LOCALAPPDATA%\devlayer` on Windows.

Hermetic means one thing in this file.
The three files are the [version file](versions.env), the [URL builder](internal/binaries/urls.go), and the [pin file](internal/sums/checksums.sha256).
The same version, the same URL, and the same sha256 produce the same prebuilt archive.
Each rung above those three files adds bytes they do not fix.
Darwin means macOS.

## Ladder

The bottom rung is the core.
Read upward.
Each rung is less fixed than the rung under it.

```text
                        less fixed
                             ^
  +---+---------+-------------------------------------+
  | 6 | host    | OS, keys, SSH, fonts, later PATH    |
  +---+---------+-------------------------------------+
  | 5 | choice  | dotfiles named in the config        |
  +---+---------+-------------------------------------+
  | 4 | runtime | wrappers in the sysroot             |
  +---+---------+-------------------------------------+
  | 3 | plugins | nvim commit, no sha256              |
  +---+---------+-------------------------------------+
  | 2 | build   | portable hermetic build             |
  +---+---------+-------------------------------------+
  | 1 | fetch   | archive sha256 must match           |
  +---+---------+-------------------------------------+
  | 0 | core    | version file, URL builder, pin file |
  +---+---------+-------------------------------------+
                             |
                        more fixed
```

## Rungs

| Rung | Fixed by | Not fixed | Code |
| --- | --- | --- | --- |
| 0 Core | version, URL, sha256 | compiled tools | [version file](versions.env) |
| 1 Fetch | archive bytes | network on each build | [download.go](internal/download/download.go) |
| 2 Build | portable hermetic build | host_sh, host_bash, host_ar, host_machine | [linux/BUILD.bazel](linux/BUILD.bazel) |
| 3 Plugins | git commit | archive bytes | [lock file](config/nvim/nvim-pack-lock.json) |
| 4 Runtime | wrapper text | host `/bin/sh`, `libSystem` | [assemble.go](internal/assemble/assemble.go) |
| 5 Choice | paths you list | a key, if you list it | [example](config/devlayer/config.toml.example) |
| 6 Host | nothing in the sysroot | OS, SSH, fonts, later PATH | [install.sh](scripts/install.sh) |

## Build paths

[build.go](cmd/build.go) picks one path.

```text
  devlayer build
        |
        +-- linux -----------> Bazel and Zig
        |
        +-- darwin ----------> hash, then host compile
        |
        +-- windows ---------> hash, no compile
```

[bazelLinuxBundle](cmd/build.go) runs when `bazel` is on `PATH` and [MODULE.bazel](MODULE.bazel) is in the source tree.
Zig builds both Linux CPUs.
A missing `bazel` or a missing [MODULE.bazel](MODULE.bazel) fails the build.
A Mac can build the Linux sysroot.
A Linux machine can build the macOS objects.
The macOS input is the pinned SDK package in [macos_sdk.bzl](tools/macos_sdk.bzl).
A container does not build the sysroot.

[extensions.bzl](tools/extensions.bzl) passes the pin file to `http_archive`.
`devlayer build` runs `//linux:bundle_<arch>` when `bazel` and [MODULE.bazel](MODULE.bazel) are present.
That target builds the Linux sysroot.
The bundle action copies those repo files.
It does not download.
`bazel build --config=rbe` sends actions to a local Buildbarn worker.
An empty image pin fails that build.
The worker must seal each action.
The Mac still runs repo rules.
[hermetic.md](hermetic.md) is that portable hermetic build.
[hermetic_cc_toolchain](MODULE.bazel) is not the default toolchain.
Each compile target selects it.

## Portable build

[hermetic.md](hermetic.md) is the portable hermetic build.
Run `bazel run //tools/hermeticgraph` to write that file.
`bazel query` is the source.
Each node names its rule.
Each node links to the build file for that rule.
A child is an earlier input.

## Kernel

The kernel reads two fields in a compiled Linux ELF.
`e_type` is `ET_EXEC` or `ET_DYN`.
`PT_INTERP` is a path, or the row is absent.

```text
  e_type    path      what the kernel does
  ET_EXEC   absent    map at the address in the row
  ET_DYN    absent    add a load offset, then jump
  either    present   open that path, then jump there
```

The third row names the dynamic linker.
These Linux compiles omit that row.
`ET_DYN` with no path is static PIE.
The entry in the file adds the load offset.
The linker writes those records on the host.
The target binary does not run during the link.

`file` prints `statically linked` for `ET_EXEC`.
`file` prints `static-pie linked` for static PIE.
Both phrases mean that the file has no dynamic linker.
The CI check accepts both phrases.
Any other phrase fails the job.

A current Ubuntu kernel loads static PIE.
A current Alpine kernel loads static PIE.
A loader that accepts only `ET_EXEC` rejects static PIE.

An earlier `-no-pie` does not override a later `-pie`.
Zig 0.16.0 writes static PIE for `-static -no-pie -fPIE -pie`.
The link line keeps `-static` and drops `-no-pie`.

The sittings in [the hermetic compile](https://github.com/stefanpenner/the-hermetic-build) name this protocol.

## How

1. The [version file](versions.env) records each tool version.
2. [urls.go](internal/binaries/urls.go) builds the archive URL from that version.
3. The [pin file](internal/sums/checksums.sha256) records the sha256 and the URL. [Must](internal/sums/sums.go) returns an error when the URL has no line.
4. [verifiedTemp](internal/download/download.go) hashes the body. A mismatch removes the temp file and returns an error.

- [TestAllURLsPinned](internal/binaries/pins_test.go) fails when a URL has no pin, or a pin has no URL. The test does not fetch the archive.
- [update-check.yml](.github/workflows/update-check.yml) has no `schedule`. The workflow comment names a NAS as the clock. This repo has no dispatcher.
- That job rewrites the version file only. It runs `bazel test //cmd/... //internal/... //tools/...`. Then it runs the [probe](tools/download/main.go).
- The probe reads HTTP status. It does not hash the body. A new URL with no pin fails the test, so the job does not merge.
- The merge uses `GITHUB_TOKEN`. That pull request does not run pull_request CI. The push to master runs [ci.yml](.github/workflows/ci.yml).
- [parse.go](internal/checkupdates/parse.go) rejects a tag that contains `rc`, `alpha`, `beta`, `nightly`, or `snapshot`. A rejected latest tag causes no bump. The tool does not pick an older stable tag.
- The linux job in [ci.yml](.github/workflows/ci.yml) accepts `statically linked` and `static-pie linked`. Another `file` phrase fails the job.
- Linux wrappers in [assemble.go](internal/assemble/assemble.go) set `GIT_EXEC_PATH`, `FPATH`, `VIMRUNTIME`, and `GOROOT` in the sysroot. Darwin and Windows wrappers are in [build.go](cmd/build.go).
- Check-updates, download, and assemble are Go programs under [tools/](tools/). The bootstrap runs before a CLI exists, so [install.sh](scripts/install.sh) and [install.ps1](scripts/install.ps1) stay shell. [workspace_status.sh](tools/workspace_status.sh) stays shell for Bazel.
- The CLI embeds the pin file and the version file ([main.go](main.go)). Bazel [9.0.1](.bazelversion) and Go 1.23.3 build that CLI ([MODULE.bazel](MODULE.bazel), [go.mod](go.mod)). The sysroot Go SDK is `GO_VERSION` in the version file.
- Darwin and Windows read that embedded version file. Rebuild the CLI before those builds see a checkout edit.
- Linux with Bazel reads the checkout version file ([defs.bzl](linux/defs.bzl)).

## Superstatic

A shipped program does not open a host data file.
This repo holds the bytes.
Each build stores that file in the program.

Terminfo is [fallbacks.txt](third_party/ncurses/fallbacks.txt).
Each ncurses build reads that file.
The build compiles those names into the library.
The library does not open a terminfo directory.
Add a terminal by adding one line to that file.

`xterm-ghostty` is not a name in ncurses 6.6.
[aliases.ti](third_party/ncurses/aliases.ti) adds that name.
The entry uses the `ghostty` description.

Neovim reads terminals through unibilium.
That library does not search the host.
An empty path makes the lookup fail.
Neovim then uses the builtin table in the pinned Neovim source.
That table is a second input.
It is not `fallbacks.txt`.

The `share/zsh` tree is a directory beside the binary.
Those shell functions are not inside the zsh binary.

## Node

[node/](node/README.md) builds a kernel, an initrd, and an EROFS of the Linux sysroot.
The kernel source and the kernel LLVM are rung 0 pins.
The Alpine build tools, mkfs.erofs, and QEMU are sha256 pins in [node/extensions.bzl](node/extensions.bzl).
The kernel action still runs host `sh`, `sed`, `awk`, `grep`, and `tar`. That is rung 2.
The kernel LLVM links the host glibc. A Linux x86_64 exec host builds the kernel.
Two kernel builds in different directories gave the same bytes.

## Limits

- Darwin nvim, htop, and eza clone a tag and compile with the host toolchain ([build.go](cmd/build.go)). Darwin btop and make hash the tarball, then the host compiler builds them.
- Delta on linux aarch64 uses the gnu triple ([platform.go](internal/platform/platform.go)). Upstream has no musl asset for delta on that arch.
- [urls.go](internal/binaries/urls.go) fetches eza on Windows only. Bazel builds eza from a pinned tag archive and vendored crates. The git feature stays off. The Darwin CLI still clones the tag and uses host cargo with vendored-libgit2. The Linux sysroot has no eza.
- The Apple cargo action copies the headers in [tools/libiconv](tools/libiconv). It compiles them with zig. It does not run configure or make. GitHub can rebuild the eza tag archive. The pin then fails the fetch.
- The version file records `GH_VERSION`. No URL fetches gh ([build.go](cmd/build.go)).
- Plugin revs are commits in the [lock file](config/nvim/nvim-pack-lock.json). [ArchiveURL](internal/nvimplugins/nvimplugins.go) says the archive bytes are not a sha256.
- [install.sh](scripts/install.sh) hashes the release only when the hash file and a hash tool both exist. A missing hash file skips the hash. [upgrade.go](cmd/upgrade.go) does not read that file. [install.ps1](scripts/install.ps1) uses the same rule.
- Publish packs the sysroot again and builds the CLI again ([ci.yml](.github/workflows/ci.yml)). The tools inside stay from the sysroot job. Nothing compares the new hash file to the sysroot archive.
- Unix install uses `~/.local` when you omit `DEVLAYER_DIR`. Other tools share that `bin` directory. The script writes the sysroot `bin` directory at the front of `PATH` in `~/.profile`.
- terminal-browser, grok, termium, and simserve are host apps. Darwin copies one host stub to `bin/terminal-browser`, `bin/grok`, `bin/agent`, `bin/termium`, and `bin/simserve`. The file name selects the app. The stub downloads the pinned bytes for that OS and CPU on the first run. It stores the tree under `~/.local/share/devlayer/host`. A stamp records the sha256. Termium setup then writes its runtime under `~/.local/share/termium`. The stub does not edit the shell rc. Linux arm64 has no Termium pin. The Linux bundle does not contain the stub. The download is not in the bundle.
- simserve is a host citizen. The pin is darwin arm64 only. The gzip is rung 0. The first run checks rung 1. The downloaded program links `libobjc`, `IOSurface`, `CoreFoundation`, `CoreGraphics`, `ImageIO`, and `libSystem`. It opens `CoreSimulator` and `SimulatorKit` by name. Those libraries stay on the Mac. They are rung 6. A later pin can drop `ImageIO`, `CoreGraphics`, and `CoreFoundation` when JPEG encode no longer uses them. The download is not in the bundle. The hermetic claim does not pass.
- Dotfile copy is optional. The copier has no key filter ([build.go](cmd/build.go)). The [example](config/devlayer/config.toml.example) says the sync list is config only, not private keys.
- [autoupdate.go](cmd/autoupdate.go) calls GitHub `releases/latest` at most once per day. It does not install.
- [MODULE.bazel.lock](MODULE.bazel.lock) is not an input of `devlayer build`. The lock can record an older tool version than the version file.
