"""QEMU and swtpm for the tests, from pinned Homebrew bottles. No install, no brew.

Each bottle is a ghcr.io blob, addressed and checked by its SHA-256.
The bottles keep @@HOMEBREW_PREFIX@@ in their load commands. The test
sets DYLD_LIBRARY_PATH to the bottle lib directories, and dyld finds each
library by its leaf name. The bytes stay as signed, so no re-signing.

Regenerate the pins: `brew info --json=v2 <formula>` gives bottle.stable.files.<tag>.
"""

def _brew_bottles_impl(rctx):
    for name, sha in rctx.attr.bottles.items():
        url = "https://ghcr.io/v2/homebrew/core/%s/blobs/sha256:%s" % (name.replace("@", "/"), sha)
        rctx.download_and_extract(
            url = url,
            sha256 = sha,
            type = "tar.gz",
            output = "cellar",
            headers = {"Authorization": ["Bearer QQ=="]},
        )
    rctx.file("BUILD.bazel", """
filegroup(
    name = "qemu",
    srcs = glob([
        "cellar/*/*/lib/**/*.dylib",
        "cellar/qemu/*/bin/qemu-system-x86_64",
        "cellar/swtpm/*/bin/swtpm",
        "cellar/qemu/*/share/qemu/*.bin",
        "cellar/qemu/*/share/qemu/*.rom",
        "cellar/qemu/*/share/qemu/edk2-x86_64-code.fd",
        "cellar/qemu/*/share/qemu/edk2-i386-vars.fd",
    ]),
    visibility = ["//visibility:public"],
)

alias(
    name = "qemu-system-x86_64",
    actual = glob(["cellar/qemu/*/bin/qemu-system-x86_64"])[0],
    visibility = ["//visibility:public"],
)

alias(
    name = "swtpm",
    actual = glob(["cellar/swtpm/*/bin/swtpm"])[0],
    visibility = ["//visibility:public"],
)

alias(
    name = "ovmf_code",
    actual = glob(["cellar/qemu/*/share/qemu/edk2-x86_64-code.fd"])[0],
    visibility = ["//visibility:public"],
)

alias(
    name = "ovmf_vars",
    actual = glob(["cellar/qemu/*/share/qemu/edk2-i386-vars.fd"])[0],
    visibility = ["//visibility:public"],
)
""")

brew_bottles = repository_rule(
    implementation = _brew_bottles_impl,
    attrs = {"bottles": attr.string_dict(doc = "formula name -> bottle SHA-256")},
)
