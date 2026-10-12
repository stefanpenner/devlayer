"""Linux tool archives. One row is one program."""

load("@rules_go//go:def.bzl", "go_test")
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

def _repo(arch):
    if arch == "x86_64":
        return "linux_amd64"
    if arch == "aarch64":
        return "linux_arm64"
    fail("bad arch " + arch)

def _bins(repo):
    return [
        ("fd", "@fd_%s//:fd" % repo),
        ("bat", "@bat_%s//:bat" % repo),
        ("rg", "@rg_%s//:rg" % repo),
        ("delta", "@delta_%s//:delta" % repo),
        ("dust", "@dust_%s//:dust" % repo),
        ("fzf", "@fzf_%s//:fzf" % repo),
        ("lazygit", "@lazygit_%s//:lazygit" % repo),
        ("glow", "//third_party/glow:glow_%s" % repo),
        ("age", "@age_%s//:age" % repo),
        ("age-keygen", "@age_%s//:age-keygen" % repo),
        ("shh", "@shh_%s//:shh" % repo),
        ("direnv", "@direnv_%s//file:direnv" % repo),
        ("jq", "@jq_%s//file:jq" % repo),
        ("batman", "@bat_extras//:bin/batman"),
    ]

def _trees(repo):
    # dest, filegroup, anchor file, dirname count
    return [
        ("go", "@go_sdk_%s//:all" % repo, "@go_sdk_%s//:bin/go" % repo, 2),
        ("zig", "@zig_%s//:all" % repo, "@zig_%s//:zig" % repo, 1),
        ("share/zsh-autosuggestions", "@zsh_autosuggestions//:all", "@zsh_autosuggestions//:zsh-autosuggestions.zsh", 1),
        ("share/zsh-fast-syntax-highlighting", "@fast_syntax_highlighting//:all", "@fast_syntax_highlighting//:fast-syntax-highlighting.plugin.zsh", 1),
        ("share/zsh-history-substring-search", "@zsh_history_substring_search//:all", "@zsh_history_substring_search//:zsh-history-substring-search.zsh", 1),
        ("share/powerlevel10k", "@powerlevel10k//:all", "@powerlevel10k//:powerlevel10k.zsh-theme", 1),
    ]

_FILES = [
    ("share/fzf/key-bindings.zsh", "@fzf_shell//:shell/key-bindings.zsh"),
    ("share/fzf/completion.zsh", "@fzf_shell//:shell/completion.zsh"),
]

def _root(anchor, ups):
    expr = '"$(execpath %s)"' % anchor
    for _ in range(ups):
        expr = "$$(dirname %s)" % expr
    return expr

def _bundle(arch):
    repo = _repo(arch)
    srcs = [":%s_%s" % (name, arch) for name in _BUNDLE]
    cmd = [
        "$(execpath //tools/assemble:assemble)",
        "--out $@",
        "--arch " + arch,
    ]
    for name in _BUNDLE:
        cmd.append("--%s $(location :%s_%s)" % (name, name, arch))
    for name, label in _bins(repo):
        srcs.append(label)
        cmd.append('--bin %s="$(execpath %s)"' % (name, label))
    for dest, group, anchor, ups in _trees(repo):
        srcs.append(group)
        srcs.append(anchor)
        cmd.append('--tree %s="%s"' % (dest, _root(anchor, ups)))
    for dest, label in _FILES:
        srcs.append(label)
        cmd.append('--file %s="$(execpath %s)"' % (dest, label))

    # gitstatusd is not in the powerlevel10k git archive. Ship the pinned
    # binary. The plugin otherwise downloads it into $HOME at login.
    native.genrule(
        name = "gitstatusd_" + arch,
        srcs = ["@gitstatusd_linux_%s//file" % arch],
        outs = ["gitstatusd-linux-" + arch],
        cmd = "mkdir -p $(RULEDIR)/gs && tar -xzf $(SRCS) -C $(RULEDIR)/gs && chmod 755 $(RULEDIR)/gs/gitstatusd-linux-%s && mv $(RULEDIR)/gs/gitstatusd-linux-%s $@" % (arch, arch),
    )
    srcs.append(":gitstatusd_" + arch)
    cmd.append(
        '--file share/powerlevel10k/gitstatus/usrbin/gitstatusd-linux-%s="$(location :gitstatusd_%s)"' % (arch, arch),
    )

    # manual keeps //... from packing this archive.
    # The action copies repo outputs. It does not fetch.
    native.genrule(
        name = "bundle_" + arch,
        srcs = srcs,
        tools = [
            "//tools/assemble:assemble",
            "//bininfo",
            "//bininfo:policy",
        ],
        outs = ["devlayer-linux-%s.tar.gz" % arch],
        cmd = "BININFO=$(location //bininfo) BININFO_POLICY=bininfo/policy/linux BININFO_POLICY_OVERLAY=bininfo/policy/linux-%s " % arch + " ".join(cmd),
        tags = ["manual"],
        visibility = ["//visibility:public"],
    )

    # loadcheck reads each file. It does not run Linux binaries on macOS.
    # A Linux host runs the static musl programs.
    # glibc programs are not executed. Alpine has no glibc.
    go_test(
        name = "smoke_" + arch,
        srcs = ["smoke_test.go"],
        data = [
            ":bundle_" + arch,
            "//bininfo",
            "//bininfo:policy",
        ],
        deps = ["//internal/loadcheck"],
        env = {
            "DEVLAYER_BUNDLE": "$(location :bundle_%s)" % arch,
            "BININFO": "$(rootpath //bininfo)",
            "BININFO_POLICY": "bininfo/policy/linux",
            "BININFO_POLICY_OVERLAY": "bininfo/policy/linux-%s" % arch,
        },
    )
