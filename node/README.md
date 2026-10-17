# node

This package builds the SER9 node image.
The image is a kernel and a tiny initrd.
The initrd holds `/init` and the runtime layer, a 484 KiB read-only EROFS root.
The node boots from that layer alone. More layers mount by content hash.
The same kernel is a local test target for devlayer binaries.

```bash
bazel build //node:oci                  # vmlinuz + initrd.img as an OCI layout
bazel build //layers:runtime //layers:dev
bazel test --config=node //node/...     # boots the node in the pinned QEMU
NODE_VM_ACCEL=tcg bazel test --config=node //node/...
```

## Machine

The machine is a Beelink SER9 Max (`SER9 Max-255-641TBW64PRO-DP/SG`).
The CPU is a Ryzen 7 H 255: Zen 4, Hawk Point, Radeon 780M.
The NIC is a Marvell/Aquantia AQtion AQC113 10GbE, PCI `1d6a:04c0`.
Two SER9 Max reviews name that chip (jb51.net and zhihu, 2025).
The Linux driver is `atlantic` (`CONFIG_AQTION`).
The driver reads its firmware from the NIC flash. The image has no firmware blob.
The board also has an AX200 Wi-Fi card. The kernel has no Wi-Fi.
The first boot on the SER9 confirms the PCI id in `/sys/bus/pci/devices`.

## Boot

```text
  efi-boot-shim ── LoadFile2 initrd ──> vmlinuz (EFI stub)
                                           |
        kernel ip=dhcp brings up the NIC ──+
                                           v
  /init (initramfs) ── loop ro ──> /runtime.erofs ──> overlay(tmpfs) ──> switch_root
                                                                             |
  /sbin/init (runtime) ── cgroup2, /run, lo, /layers/*, dropbear ──> /etc/node/hook, or the test payload
```

The kernel does DHCP itself (`ip=dhcp`). Init has no DHCP client.
The kernel does not renew the lease. A static DHCP reservation covers that.
`/proc/net/pnp` becomes `/etc/resolv.conf`.
Every write lands in RAM. A reboot restores the image.
Init is one static C file, [init/init.c](init/init.c), 35 KB.
Init reads boot options with `getenv`. The kernel passes unknown `name=value` options to PID 1.

## Kernel

The source is Linux 7.2.11, the latest stable on 2026-10-15.
The sha256 is in [checksums.sha256](../internal/sums/checksums.sha256). It matches kernel.org `sha256sums.asc`.
The config is `allnoconfig` plus [kernel/ser9.config](kernel/ser9.config).
`merge_config.sh` reports each value that did not reach `.config`. Any report fails the build.
There are no modules.

| Area | Built in |
| --- | --- |
| CPU | AMD and Intel, x2APIC, MCE, amd-pstate, `-mtune=znver4` |
| Boot | EFI stub, initrd, zstd kernel, KASLR |
| Console | 8250 serial, EFI framebuffer console, netconsole |
| Storage | NVMe, virtio-blk, loop |
| Filesystems | EROFS (lz4, deflate), overlayfs, tmpfs with xattrs, hugetlbfs, THP |
| Network | atlantic, virtio-net, kernel DHCP, IPv6, nftables NAT, bridge, veth, macvlan, macvtap, TUN |
| Containers | cgroup v2 controllers, all namespaces, seccomp, BPF, io_uring |
| KVM host | KVM, KVM_AMD (nested), KVM_INTEL, AMD IOMMU, IRQ remap, IOMMUFD, VFIO-PCI, vhost-net, vhost-vsock |
| Guest | virtio-pci, virtio console, virtio RNG, KVM guest clock |

`-mtune` changes scheduling only. `-march=znver4` would break QEMU TCG and Intel test hosts.
There is no amdgpu. A headless node does not need it. The EFI framebuffer shows the console on a monitor.
`KVM_INTEL` is only for Intel test hosts. The SER9 does not use it.

## Toolchain

The kernel compiler is the kernel.org LLVM 23.1.3 build, pinned by sha256.
Zig is not the kernel compiler.
Kbuild with `LLVM=1` runs `llvm-nm`, `llvm-objcopy`, `llvm-readelf`, and `llvm-strip`.
Zig ships clang and lld. Zig does not ship those four programs.
Zig is HOSTCC. Each kernel host program is a static musl binary from the pinned zig.
objtool links the pinned Alpine `libelf.a`, `libz.a`, and `libzstd.a`.
flex, bison, m4, bc, make, xz, and zstd are pinned Alpine musl packages.
A wrapper starts each one with the musl loader from the same packages.
The init binary is built by the pinned zig.
mkfs.erofs is the pinned Alpine erofs-utils 1.9.1.

## Reproducible

The build sets `SOURCE_DATE_EPOCH=0` and fixed `KBUILD_BUILD_*` values.
`-ffile-prefix-map` removes the build directory from the image.
Two builds in different directories gave the same vmlinuz sha256.
A Bazel build and a script build gave the same vmlinuz sha256.
mkfs.erofs gets `-T0 --all-time`, a zero UUID, and `--all-root`.
gen_init_cpio gets `-t 0`.

## Layers

A layer is one reproducible EROFS image, named by the sha256 of its bytes.
Layers live in [layers/](../layers/BUILD.bazel).

| Layer | Contents | Size |
| --- | --- | --- |
| `//layers:runtime` | init, dropbear, a small toybox, root's `authorized_keys` | 484 KiB |
| `//layers:dev` | the whole devlayer Linux sysroot (`//linux:bundle_x86_64`) | 311 MiB |

The node boots from the runtime layer only. It has no compilers, no language runtimes, no git.
The SER9 image does not contain the dev layer.

### Runtime contents

| Path | What | Why |
| --- | --- | --- |
| `/sbin/init`, `/sbin/node-layer` | the zig-built init, 35 KB | PID 1; `node-layer` mounts a layer after boot |
| `/bin/dropbearmulti` (`/sbin/dropbear`, `dropbearkey`, `scp`) | dropbear 2026.94, static musl, 419 KiB | SSH for people and for `devlayer push` |
| `/bin/toybox` and 37 links | toybox 0.8.14, 290 KiB | a shell for SSH sessions, plus `mount`, `losetup`, `sha256sum`, `ps`, `dmesg`, `reboot` and a few more |
| `/root/.ssh/authorized_keys` | https://github.com/stefanpenner.keys, pinned by sha256 | the only way in |
| `/etc/passwd`, `/etc/group` | root only, shell `/bin/sh` | dropbear looks up the user |
| empty dirs | `dev proc sys run tmp var test layers/.by-hash etc/node etc/dropbear` | mount points |

The toybox applets are in [layers/defs.bzl](../layers/defs.bzl) and [layers/toybox.miniconfig](../layers/toybox.miniconfig).
toysh has no `pwd` builtin. The devlayer wrappers call `pwd`, so the runtime has `pwd`.
toybox is built by devlayer's own toolbox script, [macos/toolbox.sh](../macos/toolbox.sh).

### SSH

The SSH server is dropbear, not OpenSSH.

- dropbear is one 419 KiB static binary. It bundles its own crypto (libtomcrypt, libtommath).
- OpenSSH needs OpenSSL or LibreSSL and zlib. It is a few MiB static, and wants a privsep user and `/var/empty`.
- dropbear builds with the pinned zig and its own `configure`, like the kernel's rung 2.

[layers/localoptions.h](../layers/localoptions.h) compiles password and PAM logins out. `-s` turns them off again at run time.
Only keys in `/root/.ssh/authorized_keys` can log in.
The keys file is the GitHub keys URL, pinned in [checksums.sha256](../internal/sums/checksums.sha256). If Stefan adds or removes a GitHub key, the fetch fails until the pin is updated. That is on purpose.
No private key is in the repo or the image.
Host keys are made on first connection (`dropbear -R`) into `/etc/dropbear` on the tmpfs. They change on every boot. Clients should not pin them; see the open decisions.
Init restarts dropbear if it exits.

### Mounting layers

```text
  /layers/.by-hash/<sha256>      read-only EROFS mount
  /layers/<name> -> .by-hash/<sha256>
```

- At boot, a virtio disk with serial `layer-<name>` mounts when the command line has `node_layer_<name>=<sha256>`.
- After boot, `node-layer <name> <sha256> <image-or-device>` does the same for a file or a disk.
- Both run `sha256sum` over the whole image and refuse a mismatch.
- A new version mounts beside the old one. The symlink swaps with `rename(2)`. Old mounts stay until reboot.
- Programs in a layer run in place, for example `/layers/dev/bin/git`. The devlayer wrappers find their tools relative to themselves.
- No layer is merged into `/`. An overlay of several layers at `/` is possible later, but each layer in its own directory has no conflicts.

### Seams for later steps

- Step 2: `//layers:dev` exists today. A smaller, chosen dev layer replaces the whole bundle there.
- Step 3, `devlayer push <layer> <host>`, not built. The seam is two commands over SSH:
  `scp dev.erofs root@node:/run/layers/<sha256>.erofs`, then
  `ssh root@node node-layer dev <sha256> /run/layers/<sha256>.erofs`.
  `node-layer` checks the hash. Nothing else is needed on the node.
- dm-verity could replace the `sha256sum` pass: the kernel checks blocks on read, and a mount costs nothing up front.
- Container runtime: not chosen, so none is in the runtime layer. It would be one more layer.

### EROFS settings

`mkimage -dedupe` hardlinks equal files. The git build installs one 3.6 MB binary under 145 names. EROFS keeps one inode.
erofs-utils `-Ededupe` stopped after 24 inodes in 1.9.1. It is not used.
Measured on the dev layer (the former whole-sysroot root):

| Compressor, 128 KiB clusters | Image | mkfs time |
| --- | --- | --- |
| lz4 | 368 MiB | 1.0 s |
| lz4hc (used) | 311 MiB | 2.5 s |
| deflate | 321 MiB | 2.7 s |
| lzma (Debian mkfs.erofs) | 232 MiB | 26.7 s |

The dev tree is 848 MiB after the hardlinks.
The pinned Alpine erofs-utils has no lzma. lzma needs a zig-built erofs-utils and `EROFS_FS_ZIP_LZMA`.

## Image versus cpio

The initrd holds `/init` and `/runtime.erofs` in one uncompressed cpio, 520 KiB.
Before the layering, the initrd held the whole sysroot as EROFS, 311 MiB, and the copy into the initramfs cost 1.7 s under TCG.
A cpio of the whole sysroot tree would put 848 MiB in tmpfs, which cannot be paged out.
EROFS pages decompress on read, and the page cache can drop them.

## Test target

`node_vm_test` in [defs.bzl](defs.bzl) takes one executable target.
The rule builds the target for linux/x86_64 musl with zig, through `linux_transition`.
The rule writes the binary and its runfiles into a per-test EROFS image.
The test image is a read-only virtio disk with serial `test`.
The `layers` attribute attaches more layers as read-only disks, for example `{"//layers:dev": "dev"}`.
vmrun hashes each one on the host and puts `node_layer_<name>=<sha256>` on the command line.
A per-test image is content-addressed and hermetic. 9p and virtiofs share a live host directory. virtiofs also needs virtiofsd, which macOS does not have.
[vmrun](vmrun/main.go) boots the kernel and the initrd with `node_test=1`.
Init runs `/test/entry` with `RUNFILES_DIR`, `TEST_SRCDIR`, and `TEST_TMPDIR` set.
Stdout and stderr come back on two virtio consoles.
The status comes back through `isa-debug-exit`.
The boot log is a test output.

| Test | Payload |
| --- | --- |
| `//node:sums_test_vm` | `//internal/sums:sums_test` |
| `//node:versions_test_vm` | `//internal/versions:versions_test` |
| `//node:sysroot_smoke_vm` | git, rg, nvim, htop, jq, zig, go from `/layers/dev`; the layer is read-only |
| `//node:kvm_vm` | opens `/dev/kvm`, `KVM_CREATE_VM` |
| `//node:ssh_vm` | no toolchains in the runtime; the pinned keys; dropbear on :22; an unknown key is refused; a key login and a pty session work |

Every test boots the runtime layer only. Every payload is a static binary on its own disk.
One test takes about 2.3 s under TCG, boot to power-off. The SSH test takes 3.3 s.
The smoke test takes 14 s. About 10 s of that is the guest hashing the 311 MiB dev layer under TCG.
The SSH test key is an ed25519 key made at build time by the pinned `dropbearkey` ([sshcheck/BUILD.bazel](sshcheck/BUILD.bazel)).
It is random per build, so that one file is not reproducible. It exists only on the test's disk.
The test adds its public half to `authorized_keys` on the tmpfs overlay, never to the layer.
TCG `-cpu max` has SVM, so `kvm_amd` loads in the guest.
`NODE_VM_ACCEL` picks the accelerator. Empty means KVM when `/dev/kvm` opens.
Apple silicon has no x86 HVF. A Mac runs TCG.

## Limits

- The kernel build runs on a Linux x86_64 exec host. A Mac needs `--config=rbe` or a remote cache for `//node:vmlinuz`.
- The kernel.org LLVM links the host glibc, libstdc++, zlib, and zstd. An Alpine RBE worker cannot run it.
- The Alpine hosttools pins are x86_64 only.
- The kernel build still uses host `sh`, `sed`, `awk`, `grep`, `tar`, and coreutils. This is rung 2, as in [HERMETICITY.md](../HERMETICITY.md).
- mkfs.erofs and gen_init_cpio run on Linux only. The VM tests then need Linux to build the images.
- Alpine removes old packages from the v3.24 mirror. A removed pin fails the fetch.
- The SER9 BIOS must have SVM on. Only the real board can show `kvm_amd` loaded.
- The SER9 BIOS must have the IOMMU on for VFIO.
- On this build box, `-accel kvm` hangs at boot. The tests ran with `NODE_VM_ACCEL=tcg`. Without it, vmrun picks KVM because `/dev/kvm` opens, and the tests time out.
- dropbear's `configure` uses the host shell, sed, and awk (rung 2).
- The layers build on a Linux x86_64 exec host, like the kernel.

## Open decisions

- Host keys. They change every boot today. Options: derive them from a per-node secret, ship them in a per-node layer, or accept that cattle have new keys and connect with `-o StrictHostKeyChecking=no` on a trusted LAN.
- Where pushed layers live. `/run/layers` is RAM and goes away on reboot. The NVMe could hold them.
- Container runtime. The kernel has cgroup v2, namespaces, overlayfs, seccomp, and nftables. Pick crun, runc, or youki as a layer.
- Persistent state. The NVMe is unused. Pick none, a scratch filesystem, or VM disks.
- dm-verity. Put each layer's root hash on the command line and verify at mount.
- lzma. Build erofs-utils with zig to get lzma: 232 MiB instead of 311 MiB for the dev layer, slower reads.
- Time. The node has no NTP client. KVM guests and TLS need a correct clock.
- `KVM_INTEL`. Drop it if no test host is Intel.

## Revert log

Everything is on the `feat/ser9-kernel` branch (PR #26). Nothing is merged or published.

- Undo the layering only: `git revert cc3c023 65c5b8c d65c359`. The node then boots the whole sysroot from the initrd again.
- Undo the whole node: close PR #26 and delete the branch. master is unchanged.
- The layering added `DROPBEAR_VERSION` to `versions.env` and two pins to `checksums.sha256` (dropbear, stefanpenner.keys). The revert removes them.
- The build box has extra apt packages from the kernel work. They are listed in `/workspace/REVERT-ser9-kernel.md` on the box. The layering installed nothing.
