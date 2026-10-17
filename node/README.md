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
  /sbin/init (runtime) ── cgroup2, /run, lo, /layers/*, node-identity, sshd ──> /etc/node/hook, or the test payload
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
| `//layers:runtime` | init, OpenSSH sshd, node-identity, a small toybox, root's `authorized_keys` | 3.95 MiB (4,141,056 B; was 484 KiB with dropbear) |
| `//layers:dev` | the whole devlayer Linux sysroot (`//linux:bundle_x86_64`) | 311 MiB |

The node boots from the runtime layer only. It has no compilers, no language runtimes, no git.
The SER9 image does not contain the dev layer.

### Runtime contents

| Path | What | Why |
| --- | --- | --- |
| `/sbin/init`, `/sbin/node-layer` | the zig-built init, 35 KB | PID 1; `node-layer` mounts a layer after boot |
| `/sbin/sshd`, `/libexec/sshd-session`, `/libexec/sshd-auth` | OpenSSH 10.6p1 with LibreSSL 4.3.3 libcrypto, static musl, 3.9 MB | SSH for people and for `devlayer push` |
| `/sbin/node-identity` | Go, go-tpm 0.9.8, static, 3.4 MB | the host key, held in the TPM; an ssh-agent socket for sshd |
| `/etc/ssh/sshd_config`, `/etc/ssh/trusted_user_ca_keys` (empty), `/etc/ssh/sshd_config.d/` | key-only config | see SSH below |
| `/bin/toybox` and 37 links | toybox 0.8.14, 290 KiB | a shell for SSH sessions, plus `mount`, `losetup`, `sha256sum`, `ps`, `dmesg`, `reboot` and a few more |
| `/root/.ssh/authorized_keys` | https://github.com/stefanpenner.keys, pinned by sha256 | the only way in |
| `/etc/passwd`, `/etc/group` | root, and `sshd` (uid 22, home `/var/empty`, shell `/bin/false`) | sshd looks up the user; `sshd` is the privsep user |
| empty dirs | `dev proc sys run tmp var/empty test layers/.by-hash etc/node etc/ssh/sshd_config.d` | mount points |

The toybox applets are in [layers/defs.bzl](../layers/defs.bzl) and [layers/toybox.miniconfig](../layers/toybox.miniconfig).
toysh has no `pwd` builtin. The devlayer wrappers call `pwd`, so the runtime has `pwd`.
toybox is built by devlayer's own toolbox script, [macos/toolbox.sh](../macos/toolbox.sh).

### SSH

The SSH server is OpenSSH 10.6p1 portable, built against LibreSSL 4.3.3 ([layers/openssh.sh](../layers/openssh.sh)).
Both tarballs are pinned by sha256 in [checksums.sha256](../internal/sums/checksums.sha256).
Before pinning, each was checked with gpg against the OpenBSD release keys: Damien Miller `7168B983815A5EEF59A4ADFD2A3F414E736060BA` for OpenSSH and Brent Cook `6F67522EC596C025B24549911FFAA0B24B708F96` for LibreSSL.
Both are built static musl by the pinned zig, in one Bazel action, with `-ffile-prefix-map` and `--build-id=none`.

- Why libcrypto: a TPM 2.0 cannot hold an Ed25519 key. The host key is ECDSA P-256, and OpenSSH `--without-openssl` only does Ed25519.
- Why LibreSSL: it is smaller than OpenSSL 3 and builds with a plain `configure`. Only its libcrypto is linked.
- No zlib (`--without-zlib`), no PAM, no PKCS#11, no FIDO keys (`--disable-security-key`), no lastlog/utmp/wtmp. `--without-pie`, because LibreSSL's static libcrypto is not PIC.
- Dropbear is gone. It cannot take a host key from an agent or a TPM.

[sshd_config](../layers/runtime/etc/ssh/sshd_config):

- Keys only: `AuthenticationMethods publickey`, `PasswordAuthentication no`, `KbdInteractiveAuthentication no`. PAM is not compiled in.
- `PermitRootLogin prohibit-password`.
- `TrustedUserCAKeys /etc/ssh/trusted_user_ca_keys`, empty in the image. A user CA key added there admits certificates it signed.
- `HostCertificate`: a drop-in in `/etc/ssh/sshd_config.d/` names the certificate. The private key stays in the TPM; sshd pairs the certificate with the agent key.
- Privsep: user `sshd`, directory `/var/empty`.

Only keys in `/root/.ssh/authorized_keys` can log in.
The keys file is the GitHub keys URL, pinned in [checksums.sha256](../internal/sums/checksums.sha256). If Stefan adds or removes a GitHub key, the fetch fails until the pin is updated. That is on purpose.
No private key is in the repo or the image.
Init restarts sshd if it exits.

### Host identity and the TPM

The node keeps no state, so its SSH host key comes from the TPM.

```text
  TPM owner seed ── CreatePrimary(fixed ECC P-256 template) ──> persistent handle 0x81008001
  node-identity agent ── /run/node-identity/agent.sock (0600) ──> sshd HostKeyAgent
                      └─ /run/node-identity/ssh_host_ecdsa_key.pub ──> sshd HostKey
```

- [node-identity](identity/main.go) is one static Go binary. It uses go-tpm 0.9.8 (`legacy/tpm2`, Google's TPM 2.0 library) and `golang.org/x/crypto/ssh/agent`. No TPM command is marshalled by hand.
- The key is a primary key from a fixed template in the owner hierarchy. A primary key is derived from the TPM's seed, so the same TPM gives the same key on every boot, even with the persistent handle cleared. The handle only saves the derivation time.
- On start it reads handle 0x81008001 and checks that the object is the expected ECC P-256 signing key. Otherwise it creates the primary key and persists it with `EvictControl`.
- It serves a read-only agent: list and sign only. Signing is ECDSA SHA-256 in the TPM. The private key never leaves the TPM.
- `node-identity pubkey` prints the `authorized_keys`-style line and the PEM public key.
- The kernel has `TCG_TPM`, `TCG_TIS`, `TCG_CRB`, and `HW_RANDOM_TPM` built in. The SER9's AMD fTPM uses CRB. `/dev/tpmrm0` is the resource-managed device node-identity uses.
- `TCG_TPM2_HMAC` is off. It encrypts the bus to a discrete TPM; an fTPM has no bus, and it costs boot time.
- Why not tpm2-tss and tpm2-pkcs11: they bring a C stack (tss2-esys, -mu, -tcti, sqlite for the PKCS#11 store) into the runtime. go-tpm is one binary, and sshd talks to it through the agent protocol it already has.

#### Fail closed

There is no fallback to a RAM key by default.

- No `/dev/tpmrm0`: node-identity exits, sshd does not start, and this line goes to the console, the serial line, and the kernel log:
  `identity: FATAL no TPM 2.0 (/dev/tpmrm0 missing): enable fTPM in BIOS (Security > AMD fTPM). sshd not started`
- A TPM that fails to create or load the key gives `identity: FATAL TPM ...` with the TPM response code, then `sshd not started`.
- No answer within 30 s gives `identity: FATAL the TPM did not answer in 30 s. sshd not started`.
- The node stays up. Init does not panic or reboot, and the console still works.
- The only way to get a RAM key is the kernel command line option `node_identity=ephemeral`. node-identity then logs a WARNING to the console and the kernel log, and the key changes every boot. It is for tests and development.
  The option is `node_identity`, not `node.identity`: the kernel passes `name=value` options to init's environment only when the name has no dot.

#### Follow-up: step-ca host certificates

The plan is a step-ca SSH host CA ([penner.network#15](https://github.com/stefanpenner/penner.network/issues/15)).
The node would send its TPM public key, prove possession (for example TPM attestation through step-ca's ACME device-attest or a signed challenge), and get a host certificate back.
The certificate goes in a drop-in in `/etc/ssh/sshd_config.d/` with `HostCertificate`. Clients trust `@cert-authority` instead of each node's key.
User certificates work the same way through `TrustedUserCAKeys`.
`//node:ssh_vm` already checks both with a throwaway CA made inside the VM.

#### Installer

The installer stick's log should gain a TPM presence check: whether `/dev/tpmrm0` exists, and the TPM's manufacturer and firmware version. A node without fTPM enabled then shows up at install time, not at first SSH.

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

The initrd holds `/init` and `/runtime.erofs` in one uncompressed cpio, 3.99 MiB (520 KiB with dropbear).
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
| `//node:ssh_vm` | swtpm; no toolchains or ssh client in the runtime; the pinned keys; sshd on :22; an unknown key is refused; password and keyboard-interactive are refused; a key login and a pty session work; the served host key is the TPM key; a user certificate logs in; a host certificate verifies |
| `//node:identity_tpm_vm` | the same checks over two boots on one swtpm state directory: the host key is the same, and nothing from boot 1 is left in boot 2 |
| `//node:no_tpm_vm` | no TPM: sshd is not listening, and the exact FATAL line is in the kernel log and on serial |
| `//node:ephemeral_vm` | `node_identity=ephemeral`, no TPM: the WARNING is on serial, key login works, the host key differs between two boots |

Every test boots the runtime layer only. Every payload is a static binary on its own disk.
One test takes about 2.5 s under TCG, boot to power-off. The SSH test takes 4 s, 4.5 s per boot for the two-boot tests.
The smoke test takes 14 s. About 10 s of that is the guest hashing the 311 MiB dev layer under TCG.
The SSH test keys and test CAs are made inside the VM by the pinned `ssh-keygen`, on the tmpfs. No key is in the repo or on any disk image.
The test adds its public key to `authorized_keys` on the tmpfs overlay, never to the layer.
`ssh` and `ssh-keygen` come from the same OpenSSH build and ride on the test disk. The runtime layer has no ssh client.
TPM tests run swtpm 0.10.2, pinned like QEMU (Alpine packages on Linux, Homebrew bottles on macOS), with `-tpmdev emulator` and `-device tpm-crb`.
vmrun keeps the swtpm state directory across the boots of one test.
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
- OpenSSH's and LibreSSL's `configure` use the host shell, sed, and awk (rung 2).
- x/crypto is v0.40.0, the last release that builds with rules_go's Go 1.23.3 SDK. v0.45.0 fixed a panic on malformed ssh-agent messages. The agent socket is root-only (0600) and its only client is sshd, so the risk is low. Bumping the SDK lifts the pin.
- The swtpm macOS bottles are pinned but not run on a Mac yet.
- The TPM create/load failure path has no VM test. swtpm has no switch to make CreatePrimary fail.
- Only the SER9 can show: fTPM enabled in the BIOS (Security > AMD fTPM), the CRB interface probing, `/dev/tpmrm0` present, and key create time on the fTPM. A BIOS fTPM clear changes the owner seed, and so the host key.
- The layers build on a Linux x86_64 exec host, like the kernel.

## Open decisions

- Host key trust. The TPM key is stable per board. Until step-ca (penner.network#15), clients pin each node's key on first use.
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
- Undo OpenSSH and the TPM identity only: `git revert` the commits listed in `/workspace/REVERT-ser9-kernel.md`, newest first. The runtime then has dropbear again.
- The layering added `DROPBEAR_VERSION` to `versions.env` and two pins to `checksums.sha256` (dropbear, stefanpenner.keys). The revert removes them. The OpenSSH step replaced it with `OPENSSH_VERSION` and `LIBRESSL_VERSION` and the dropbear pin with the openssh and libressl pins. go.mod gained go-tpm and x/crypto; x/sys moved to v0.34.0.
- The build box has extra apt packages from the kernel work. They are listed in `/workspace/REVERT-ser9-kernel.md` on the box. The layering installed nothing.
