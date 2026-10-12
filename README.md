# devlayer

devlayer has two jobs.
The first job is a portable hermetic build.
The second job installs a sysroot.
A build target, a test target, and a run target use that sysroot.

The CLI builds the sysroot.
The CLI installs the sysroot.
The CLI copies the files you list.

The sysroot is `$DEVLAYER_PREFIX` or `~/.local` on Unix.
The sysroot is `%LOCALAPPDATA%\devlayer` on Windows.

## What is here

The sysroot has tools and wrappers.

Linux compiles `git`, `zsh`, `nvim`, `htop`, `btop`, and `make` with Zig.
The pin file records the other Linux archives.
The macOS sysroot has no `zsh` binary and no `git`.
The Linux sysroot has no `eza`.
Bazel can build `eza` for Darwin and for Linux musl.
That build turns the git feature off.
The git feature needs libgit2 and openssl.
Those libraries are not pinned here.
No build fetches `gh`.
A Darwin build compiles with Zig and the pinned SDK.
`cmake` and `make` for the Darwin CLI still come from the host.
`cargo` for `bininfo` and for Bazel `eza` is pinned.
The Apple cargo action copies three libiconv headers from `tools/libiconv`.
That action compiles them with zig.
It does not call `configure` or `make`.
The Darwin CLI still clones `eza` and uses host `cargo`.
`codesign` stays on the Mac.
A Windows build uses pinned archives.

Wrappers set `GIT_EXEC_PATH`, `FPATH`, `VIMRUNTIME`, and `GOROOT`.
`config/nvim/nvim-pack-lock.json` names each plugin commit.
The plugin archive has no sha256.
`versions.env` names each tool version.

## How to use

Install the CLI on Linux or macOS.

```bash
curl -fsSL https://raw.githubusercontent.com/stefanpenner/devlayer/master/scripts/install.sh | bash
```

Install the CLI on Windows.

```powershell
irm https://raw.githubusercontent.com/stefanpenner/devlayer/master/scripts/install.ps1 | iex
```

Build the sysroot for this OS.
Install the sysroot.

```bash
devlayer build --os darwin
devlayer install
```

`devlayer build --os linux` runs Bazel.
Zig builds both Linux CPUs.
The command needs `bazel` on `PATH` and `MODULE.bazel` in the checkout.
A Mac can build that Linux sysroot.

`bazel build //linux:bundle_aarch64 --config=rbe` sends actions to a local Buildbarn worker.
The address is `grpc://127.0.0.1:8980`.
The Mac runs repo rules, Starlark, and bzlmod.
The worker runs the compile.
The worker image is Alpine 3.22.1.
`rbe/image.bzl` pins one digest per CPU.
The digest is the registry manifest.
This repo does not boot a microVM.
Firecracker needs Linux KVM.
This machine is macOS.
No worker process lives in this repo.
The build fails at the connection until a worker listens.
The config sets `--spawn_strategy=remote`.
The config does not allow a local fallback.
The worker must advertise `OSFamily=Linux` and the same `container-image`.
The runner container has no NIC.
The worker must seal each action with linux-sandbox.
That seal is private mounts and a private process list.
This repo cannot check that seal.
`codesign` stays on the Mac.
Do not use `--config=rbe` for that step.
A Linux machine can build the macOS objects.
The macOS input is the pinned SDK package.
A Windows build does not compile.

Write the sync list.
Edit the list.
Do not list a private key.

## Private layer

House tools stay out of this repo.
Write `~/.config/devlayer/private.toml` on a machine that needs them.

```toml
[private]
repo = "git@github.com:you/private.git"
path = "~/.ai-private"
```

`devlayer build` clones that checkout.
The command writes `devlayer-private.tar.gz`.
The archive omits `.git`.
`devlayer install` and `devlayer push` extract it under `$HOME`.
They run `$HOME/.ai-private/install.sh` when that file exists.
No `private.toml` means no private layer.
Do not commit `private.toml`.
Do not commit the private archive.

```bash
devlayer init
```

```toml
[dotfiles]
sync = [
  ".zshrc",
  ".config/nvim",
  ".gitconfig",
  ".ssh/config",
]
```

Send the sysroot to a host.
`nas` must work in `ssh nas`.

```bash
devlayer build
devlayer push nas
devlayer status nas
```

- `devlayer ls` lists tools, dotfiles, and plugins.
- `devlayer doctor` checks the local install.
- `devlayer upgrade` installs the latest CLI release.
- `devlayer versions` prints the tool versions.
- `devlayer check-updates` rewrites `versions.env`.

## How to extend

Add a compiled Linux tool.

1. Add the source archive in `tools/extensions.bzl`.
2. Add the sha256 line in `internal/sums/checksums.sha256`.
3. Add `third_party/<name>/BUILD.bazel`.
4. Add one `_CHAIN` row in `linux/defs.bzl`.
5. Add the name to `_BUNDLE` when the sysroot ships that tool.

`_BUNDLE` names the tools in the sysroot.
`//linux:bundle_<arch>` is the Linux sysroot target.
The bundle action copies pinned repo files.
It does not download.
Add a prebuilt label in `_bins`, `_trees`, or `_FILES` in `linux/defs.bzl`.

Add a pinned archive.

1. Add the version in `versions.env`.
2. Add the URL in `internal/binaries/urls.go`.
3. Add the sha256 line in `internal/sums/checksums.sha256`.
4. Add the name in `internal/binaries/fetch.go`.

`zlib` and `ncurses` stay in `_CHAIN`.
They are not in `_BUNDLE`.

## Portable build

[hermetic.md](hermetic.md) is the portable hermetic build.
Run `bazel run //tools/hermeticgraph` to write that file.
`bazel query` is the source.
Each node names its rule.
Each node links to the build file for that rule.
[HERMETICITY.md](HERMETICITY.md) has the ladder, the kernel, and the limits.
