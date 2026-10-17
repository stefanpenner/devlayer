# node

This package builds the SER9 node image.
The image is a kernel, a tiny initrd, and the devlayer Linux sysroot.
The sysroot is a read-only EROFS root.
The same kernel is a local test target for devlayer binaries.

```bash
bazel build //node:oci                  # vmlinuz + initrd.img as an OCI layout
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
  /init (initramfs) ── loop ro ──> /sysroot.erofs ──> overlay(tmpfs) ──> switch_root
                                                                             |
  /sbin/init (sysroot) ── cgroup2, /run ──> /etc/node/hook, or the test payload
```

The kernel does DHCP itself (`ip=dhcp`). Init has no DHCP client.
The kernel does not renew the lease. A static DHCP reservation covers that.
`/proc/net/pnp` becomes `/etc/resolv.conf`.
Every write lands in RAM. A reboot restores the image.
Init is one static C file, [init/init.c](init/init.c), 29 KB.
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

## Sysroot image

The EROFS root is `//linux:bundle_x86_64`, the devlayer Linux sysroot.
The node adds `/sbin/init`, `/bin/sh` (a link to zsh), `/etc/passwd`, `/etc/group`, and empty mount points.
The git build installs one 3.6 MB binary under 145 names.
`mkimage -dedupe` hardlinks equal files. EROFS keeps one inode.
erofs-utils `-Ededupe` stopped after 24 inodes in 1.9.1. It is not used.

| Compressor, 128 KiB clusters | Image | mkfs time |
| --- | --- | --- |
| lz4 | 368 MiB | 1.0 s |
| lz4hc (used) | 311 MiB | 2.5 s |
| deflate | 321 MiB | 2.7 s |
| lzma (Debian mkfs.erofs) | 232 MiB | 26.7 s |

The tree is 848 MiB after the hardlinks.
The pinned Alpine erofs-utils has no lzma. lzma needs a zig-built erofs-utils and `EROFS_FS_ZIP_LZMA`.

## Image versus cpio

The initrd holds `/init` and `/sysroot.erofs` in one uncompressed cpio.
The kernel copies the cpio into the initramfs. That copy costs 1.7 s under TCG.
EROFS pages decompress on read. The page cache can drop them.
A cpio of the whole tree would put 848 MiB in tmpfs. tmpfs pages cannot be dropped.
That cpio would take about 4.6 s to unpack under TCG, by the same rate. That number is an estimate.

## Test target

`node_vm_test` in [defs.bzl](defs.bzl) takes one executable target.
The rule builds the target for linux/x86_64 musl with zig, through `linux_transition`.
The rule writes the binary and its runfiles into a per-test EROFS image.
The test image is the first virtio disk, read-only.
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
| `//node:sysroot_smoke_vm` | git, rg, nvim, htop, jq, zig, go from the EROFS root |
| `//node:kvm_vm` | opens `/dev/kvm`, `KVM_CREATE_VM` |

One test takes about 4 s under TCG. Most of that is the initramfs copy.
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
- The 311 MiB initrd goes through the shim. The shim cache must fit it on the ESP.
- On this build box, `-accel kvm` hangs at boot. The tests ran with TCG.
