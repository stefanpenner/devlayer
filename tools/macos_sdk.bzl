"""Pinned Apple macOS SDK.

The URL is Command Line Tools 27.0.
The package id is com.apple.pkg.CLTools_SDK_macOS_NMOS.
That package holds MacOSX27.0.sdk.
Bazel checks the sha256. unpacksdk writes sdk.tar.
The tar top directory is MacOSX.sdk.
"""

# Product 082-83364. English dist version 27.0.0.0.1788430688.
_URL = "https://swcdn.apple.com/content/downloads/58/48/082-83364-A_KCEBOO2NJS/0d2rj4y5ucjlkgcvqt6f6a4pergug5tu2b/CLTools_macOSNMOS_SDK.pkg"
_SHA256 = "d55351824fd17742fd6e1e7a252fa1028698f32147ee48be9a6b75442bf30575"
_TOOL = str(Label("//tools/unpacksdk"))

def _macos_sdk_impl(rctx):
    rctx.download(
        url = _URL,
        sha256 = _SHA256,
        output = "sdk.pkg",
    )
    rctx.file("BUILD.bazel", _BUILD)

_BUILD = """
genrule(
    name = "sdk_tar",
    srcs = ["sdk.pkg"],
    outs = ["sdk.tar"],
    cmd = "$(execpath {tool}) --pkg $< --out $@",
    tools = ["{tool}"],
    visibility = ["//visibility:public"],
)
""".format(tool = _TOOL)

macos_sdk = repository_rule(
    implementation = _macos_sdk_impl,
)
