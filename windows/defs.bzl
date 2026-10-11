"""Windows tool archives. Zig cross-compiles. The host does not run the PE."""

load("//linux:cc.bzl", "flat_archive")

def windows_targets(arch):
    """Generate Windows tool targets for one target arch."""
    flat_archive(
        name = "make_" + arch,
        src = "//third_party/make:gnumake",
        member = "make.exe",
        arch = arch,
        os = "windows",
    )
