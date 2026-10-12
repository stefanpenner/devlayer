"""Pinned Alpine 3.22.1 worker userland.

One digest per CPU.
The digest is the registry manifest for that CPU.
The layer blobs were not hashed again on this machine.
The image is musl and busybox.
It is the worker userland.
It is not a Firecracker kernel.
Firecracker needs Linux KVM.
This machine is macOS.
This repo does not boot a microVM.
"""

def _check(image):
    mark = "@sha256:"
    if not image.startswith("docker://") or mark not in image:
        fail("image pin must be docker://REGISTRY/NAME@sha256:HEX")
    hexpart = image.split(mark)[1]
    if len(hexpart) != 64:
        fail("image sha256 must be 64 hex chars")
    for c in hexpart.elems():
        if c not in "0123456789abcdef":
            fail("image sha256 must be hex")
    return image

# linux/arm64 v8 manifest. Index digest is sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1.
RBE_IMAGE_ARM64 = _check("docker://docker.io/library/alpine@sha256:4562b419adf48c5f3c763995d6014c123b3ce1d2e0ef2613b189779caa787192")

# linux/amd64 manifest. Same index.
RBE_IMAGE_AMD64 = _check("docker://docker.io/library/alpine@sha256:eafc1edb577d2e9b458664a15f23ea1c370214193226069eb22921169fc7e43f")

def rbe_images():
    """Return the pin for each Linux CPU name."""
    return {
        "aarch64": RBE_IMAGE_ARM64,
        "x86_64": RBE_IMAGE_AMD64,
    }
