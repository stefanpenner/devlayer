"""Linux tool archives. One row is one program."""

load(":cc.bzl", "flat_archive", "tree_archive")

# name, kind, src, leaf. leaf is the archive member or the install root.
_CHAIN = [
    ("zlib", "flat", "//third_party/zlib", "libz.a"),
    ("ncurses", "flat", "//third_party/ncurses", "libncursesw.a"),
    ("make", "flat", "//third_party/make:gnumake", "make"),
    ("htop", "flat", "//third_party/htop", "htop"),
    ("zsh", "tree", "//third_party/zsh", "third_party/zsh/zsh"),
    ("git", "tree", "//third_party/git", "third_party/git/git"),
    ("btop", "flat", "//third_party/btop", "btop"),
    ("nvim", "tree", "//third_party/nvim:nvim_musl", "third_party/nvim/nvim_musl"),
]

# Shipped programs. zlib and ncurses are linked in. They are not bundle members.
_BUNDLE = ["git", "zsh", "htop", "btop", "nvim", "make"]

def _archive(name, kind, src, leaf, arch):
    if kind == "flat":
        flat_archive(name = name, src = src, member = leaf, arch = arch)
    elif kind == "tree":
        tree_archive(name = name, src = src, root = leaf, arch = arch)
    else:
        fail(kind)

def linux_targets(arch):
    """Archives for one Linux CPU, then the bundle."""
    for name, kind, src, leaf in _CHAIN:
        _archive(name + "_" + arch, kind, src, leaf, arch)
    _bundle(arch)

def linux_chain(arch):
    """One filegroup for //chain:linux_<arch>."""
    native.filegroup(
        name = "linux_" + arch,
        srcs = ["//linux:%s_%s" % (name, arch) for name, _, _, _ in _CHAIN],
        visibility = ["//visibility:public"],
    )

def _bundle(arch):
    native.genrule(
        name = "bundle_" + arch,
        srcs = [":%s_%s" % (name, arch) for name in _BUNDLE] + ["//:versions.env"],
        tools = ["//tools/assemble:assemble"],
        outs = ["devlayer-linux-%s.tar.gz" % arch],
        cmd = " ".join(
            [
                "$(execpath //tools/assemble:assemble)",
                "--out $@",
                "--arch " + arch,
                "--versions $(location //:versions.env)",
            ] + [
                "--%s $(location :%s_%s)" % (name, name, arch)
                for name in _BUNDLE
            ],
        ),
        # assemble still fetches pinned prebuilts. The compile inputs do not.
        tags = ["manual", "no-sandbox", "requires-network", "no-remote"],
        visibility = ["//visibility:public"],
    )
