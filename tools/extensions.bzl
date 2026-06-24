"""Module extension for fetching all third-party tool sources and pre-built binaries."""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")
load("@versions//:versions.bzl", "VERSIONS")

# ─── Build file content templates ─────────────────────────────────────────────

_ALL_SRCS = """\
filegroup(
    name = "all_srcs",
    srcs = glob(["**"]),
    visibility = ["//visibility:public"],
)
"""

_FILEGROUP_ALL = """\
filegroup(
    name = "all",
    srcs = glob(["**"]),
    visibility = ["//visibility:public"],
)
"""

def _exports(names):
    return 'exports_files([{}], visibility = ["//visibility:public"])'.format(
        ", ".join(['"%s"' % n for n in names]),
    )

def _tool_repos_impl(module_ctx):
    v = VERSIONS

    # ═══════════════════════════════════════════════════════════════════════════
    # Source builds (compiled via rules_foreign_cc)
    # ═══════════════════════════════════════════════════════════════════════════

    http_archive(
        name = "btop_src",
        urls = ["https://github.com/aristocratos/btop/archive/refs/tags/v{}.tar.gz".format(v["BTOP"])],
        strip_prefix = "btop-{}".format(v["BTOP"]),
        build_file_content = _ALL_SRCS,
        patch_cmds = [
            "sed -i.bak 's/cmake_minimum_required(VERSION 3.25)/cmake_minimum_required(VERSION 3.20)/' CMakeLists.txt",
            """sed -i.bak 's/\\$<LINK_LIBRARY:FRAMEWORK,CoreFoundation>/"-framework CoreFoundation"/g; s/\\$<LINK_LIBRARY:FRAMEWORK,IOKit>/"-framework IOKit"/g' CMakeLists.txt""",
            "sed -i.bak '/add_subdirectory.*tests/d' CMakeLists.txt",
            "sed -i.bak 's/--fatal-warnings//g' CMakeLists.txt",
        ],
    )

    http_archive(
        name = "htop_src",
        urls = ["https://github.com/htop-dev/htop/archive/refs/tags/{}.tar.gz".format(v["HTOP"])],
        strip_prefix = "htop-{}".format(v["HTOP"]),
        build_file_content = _ALL_SRCS,
    )

    http_archive(
        name = "make_src",
        urls = ["https://ftp.gnu.org/gnu/make/make-{}.tar.gz".format(v["MAKE"])],
        strip_prefix = "make-{}".format(v["MAKE"]),
        build_file_content = _ALL_SRCS,
    )

    http_archive(
        name = "ncurses_src",
        urls = ["https://ftp.gnu.org/gnu/ncurses/ncurses-{}.tar.gz".format(v["NCURSES"])],
        strip_prefix = "ncurses-{}".format(v["NCURSES"]),
        build_file_content = _ALL_SRCS,
    )

    http_archive(
        name = "git_src",
        urls = ["https://mirrors.edge.kernel.org/pub/software/scm/git/git-{}.tar.xz".format(v["GIT"])],
        strip_prefix = "git-{}".format(v["GIT"]),
        build_file_content = _ALL_SRCS,
    )

    http_archive(
        name = "zsh_src",
        # zsh.org keeps only the current release in /pub; older releases move
        # to /pub/old (the bare /pub/zsh-5.9.tar.xz URL now 404s).
        urls = ["https://www.zsh.org/pub/old/zsh-{}.tar.xz".format(v["ZSH"])],
        strip_prefix = "zsh-{}".format(v["ZSH"]),
        build_file_content = _ALL_SRCS,
    )

    # ═══════════════════════════════════════════════════════════════════════════
    # Pre-built Rust tools (per-platform archives)
    # ═══════════════════════════════════════════════════════════════════════════

    # fd — all platforms have musl/darwin builds
    for plat, target in [
        ("linux_amd64", "x86_64-unknown-linux-musl"),
        ("linux_arm64", "aarch64-unknown-linux-musl"),
        ("darwin_arm64", "aarch64-apple-darwin"),
    ]:
        name = "fd-v{ver}-{t}".format(ver = v["FD"], t = target)
        http_archive(
            name = "fd_" + plat,
            urls = ["https://github.com/sharkdp/fd/releases/download/v{ver}/{name}.tar.gz".format(
                ver = v["FD"],
                name = name,
            )],
            strip_prefix = name,
            build_file_content = _exports(["fd"]),
        )

    # bat — same pattern as fd
    for plat, target in [
        ("linux_amd64", "x86_64-unknown-linux-musl"),
        ("linux_arm64", "aarch64-unknown-linux-musl"),
        ("darwin_arm64", "aarch64-apple-darwin"),
    ]:
        name = "bat-v{ver}-{t}".format(ver = v["BAT"], t = target)
        http_archive(
            name = "bat_" + plat,
            urls = ["https://github.com/sharkdp/bat/releases/download/v{ver}/{name}.tar.gz".format(
                ver = v["BAT"],
                name = name,
            )],
            strip_prefix = name,
            build_file_content = _exports(["bat"]),
        )

    # ripgrep — linux arm64 uses gnu (no musl build)
    for plat, target in [
        ("linux_amd64", "x86_64-unknown-linux-musl"),
        ("linux_arm64", "aarch64-unknown-linux-gnu"),
        ("darwin_arm64", "aarch64-apple-darwin"),
    ]:
        name = "ripgrep-{ver}-{t}".format(ver = v["RG"], t = target)
        http_archive(
            name = "rg_" + plat,
            urls = ["https://github.com/BurntSushi/ripgrep/releases/download/{ver}/{name}.tar.gz".format(
                ver = v["RG"],
                name = name,
            )],
            strip_prefix = name,
            build_file_content = _exports(["rg"]),
        )

    # delta — linux arm64 uses gnu (no musl build)
    for plat, target in [
        ("linux_amd64", "x86_64-unknown-linux-musl"),
        ("linux_arm64", "aarch64-unknown-linux-gnu"),
        ("darwin_arm64", "aarch64-apple-darwin"),
    ]:
        name = "delta-{ver}-{t}".format(ver = v["DELTA"], t = target)
        http_archive(
            name = "delta_" + plat,
            urls = ["https://github.com/dandavison/delta/releases/download/{ver}/{name}.tar.gz".format(
                ver = v["DELTA"],
                name = name,
            )],
            strip_prefix = name,
            build_file_content = _exports(["delta"]),
        )

    # dust — darwin arm64 uses x86_64 (no native arm64 build)
    for plat, target in [
        ("linux_amd64", "x86_64-unknown-linux-musl"),
        ("linux_arm64", "aarch64-unknown-linux-musl"),
        ("darwin_arm64", "x86_64-apple-darwin"),
    ]:
        name = "dust-v{ver}-{t}".format(ver = v["DUST"], t = target)
        http_archive(
            name = "dust_" + plat,
            urls = ["https://github.com/bootandy/dust/releases/download/v{ver}/{name}.tar.gz".format(
                ver = v["DUST"],
                name = name,
            )],
            strip_prefix = name,
            build_file_content = _exports(["dust"]),
        )

    # eza — built from source via genrule (no pre-built macOS binaries)
    # See //third_party/eza for the genrule target.

    # ═══════════════════════════════════════════════════════════════════════════
    # Pre-built Go tools
    # ═══════════════════════════════════════════════════════════════════════════

    # fzf — binary at tar root (no subdirectory)
    for plat, os_arch in [
        ("linux_amd64", "linux_amd64"),
        ("linux_arm64", "linux_arm64"),
        ("darwin_arm64", "darwin_arm64"),
    ]:
        http_archive(
            name = "fzf_" + plat,
            urls = ["https://github.com/junegunn/fzf/releases/download/v{ver}/fzf-{ver}-{oa}.tar.gz".format(
                ver = v["FZF"],
                oa = os_arch,
            )],
            build_file_content = _exports(["fzf"]),
        )

    # lazygit
    for plat, lazygit_os, arch in [
        ("linux_amd64", "Linux", "x86_64"),
        ("linux_arm64", "Linux", "arm64"),
        ("darwin_arm64", "Darwin", "arm64"),
    ]:
        http_archive(
            name = "lazygit_" + plat,
            urls = ["https://github.com/jesseduffield/lazygit/releases/download/v{ver}/lazygit_{ver}_{os}_{arch}.tar.gz".format(
                ver = v["LAZYGIT"],
                os = lazygit_os,
                arch = arch,
            )],
            build_file_content = _exports(["lazygit"]),
        )

    # age — contains age/ directory with age + age-keygen
    for plat, os_name, goarch in [
        ("linux_amd64", "linux", "amd64"),
        ("linux_arm64", "linux", "arm64"),
        ("darwin_arm64", "darwin", "arm64"),
    ]:
        http_archive(
            name = "age_" + plat,
            urls = ["https://github.com/FiloSottile/age/releases/download/v{ver}/age-v{ver}-{os}-{arch}.tar.gz".format(
                ver = v["AGE"],
                os = os_name,
                arch = goarch,
            )],
            strip_prefix = "age",
            build_file_content = _exports(["age", "age-keygen"]),
        )

    # ═══════════════════════════════════════════════════════════════════════════
    # Single-binary tools (http_file)
    # ═══════════════════════════════════════════════════════════════════════════

    # direnv
    for plat, os_arch in [
        ("linux_amd64", "linux-amd64"),
        ("linux_arm64", "linux-arm64"),
        ("darwin_arm64", "darwin-arm64"),
    ]:
        http_file(
            name = "direnv_" + plat,
            urls = ["https://github.com/direnv/direnv/releases/download/v{ver}/direnv.{oa}".format(
                ver = v["DIRENV"],
                oa = os_arch,
            )],
            downloaded_file_path = "direnv",
            executable = True,
        )

    # jq — macOS uses "macos" in URL, not "darwin"
    for plat, jq_os, goarch in [
        ("linux_amd64", "linux", "amd64"),
        ("linux_arm64", "linux", "arm64"),
        ("darwin_arm64", "macos", "arm64"),
    ]:
        http_file(
            name = "jq_" + plat,
            urls = ["https://github.com/jqlang/jq/releases/download/jq-{ver}/jq-{os}-{arch}".format(
                ver = v["JQ"],
                os = jq_os,
                arch = goarch,
            )],
            downloaded_file_path = "jq",
            executable = True,
        )

    # ═══════════════════════════════════════════════════════════════════════════
    # SDKs / runtimes (full directory trees)
    # ═══════════════════════════════════════════════════════════════════════════

    # Neovim pre-built
    for plat, nvim_os, arch in [
        ("linux_amd64", "linux", "x86_64"),
        ("darwin_arm64", "macos", "arm64"),
    ]:
        archive = "nvim-{os}-{arch}".format(os = nvim_os, arch = arch)
        http_archive(
            name = "nvim_" + plat,
            urls = ["https://github.com/neovim/neovim/releases/download/v{ver}/{archive}.tar.gz".format(
                ver = v["NVIM"],
                archive = archive,
            )],
            strip_prefix = archive,
            build_file_content = _FILEGROUP_ALL,
        )

    # Go SDK
    for plat, os_name, goarch in [
        ("linux_amd64", "linux", "amd64"),
        ("linux_arm64", "linux", "arm64"),
        ("darwin_arm64", "darwin", "arm64"),
    ]:
        http_archive(
            name = "go_sdk_" + plat,
            urls = ["https://go.dev/dl/go{ver}.{os}-{arch}.tar.gz".format(
                ver = v["GO"],
                os = os_name,
                arch = goarch,
            )],
            strip_prefix = "go",
            build_file_content = _FILEGROUP_ALL,
        )

    # Zig compiler
    for plat, arch, zig_os in [
        ("linux_amd64", "x86_64", "linux"),
        ("linux_arm64", "aarch64", "linux"),
        ("darwin_arm64", "aarch64", "macos"),
    ]:
        prefix = "zig-{arch}-{os}-{ver}".format(arch = arch, os = zig_os, ver = v["ZIG"])
        http_archive(
            name = "zig_" + plat,
            urls = ["https://ziglang.org/download/{ver}/{prefix}.tar.xz".format(
                ver = v["ZIG"],
                prefix = prefix,
            )],
            strip_prefix = prefix,
            build_file_content = _FILEGROUP_ALL,
        )

    # ═══════════════════════════════════════════════════════════════════════════
    # Shell assets (plugins, themes, scripts)
    # ═══════════════════════════════════════════════════════════════════════════

    # bat-extras (batman shell script)
    http_archive(
        name = "bat_extras",
        urls = ["https://github.com/eth-p/bat-extras/releases/download/v{ver}/bat-extras-{ver}.zip".format(
            ver = v["BAT_EXTRAS"],
        )],
        build_file_content = _FILEGROUP_ALL,
    )

    # fzf shell integration (key-bindings.zsh, completion.zsh)
    http_archive(
        name = "fzf_shell",
        urls = ["https://github.com/junegunn/fzf/archive/refs/tags/v{}.tar.gz".format(v["FZF"])],
        strip_prefix = "fzf-{}".format(v["FZF"]),
        build_file_content = """\
filegroup(
    name = "shell_scripts",
    srcs = [
        "shell/key-bindings.zsh",
        "shell/completion.zsh",
    ],
    visibility = ["//visibility:public"],
)
""",
    )

    # Zsh plugins
    http_archive(
        name = "zsh_autosuggestions",
        urls = ["https://github.com/zsh-users/zsh-autosuggestions/archive/refs/tags/{}.tar.gz".format(
            v["ZSH_AUTOSUGGESTIONS"],
        )],
        strip_prefix = "zsh-autosuggestions-{}".format(v["ZSH_AUTOSUGGESTIONS"].lstrip("v")),
        build_file_content = _FILEGROUP_ALL,
    )

    http_archive(
        name = "fast_syntax_highlighting",
        urls = ["https://github.com/zdharma-continuum/fast-syntax-highlighting/archive/refs/tags/{}.tar.gz".format(
            v["FAST_SYNTAX_HIGHLIGHTING"],
        )],
        strip_prefix = "fast-syntax-highlighting-{}".format(v["FAST_SYNTAX_HIGHLIGHTING"].lstrip("v")),
        build_file_content = _FILEGROUP_ALL,
    )

    http_archive(
        name = "zsh_history_substring_search",
        urls = ["https://github.com/zsh-users/zsh-history-substring-search/archive/refs/tags/{}.tar.gz".format(
            v["ZSH_HISTORY_SUBSTRING_SEARCH"],
        )],
        strip_prefix = "zsh-history-substring-search-{}".format(v["ZSH_HISTORY_SUBSTRING_SEARCH"].lstrip("v")),
        build_file_content = _FILEGROUP_ALL,
    )

    http_archive(
        name = "powerlevel10k",
        urls = ["https://github.com/romkatv/powerlevel10k/archive/refs/tags/{}.tar.gz".format(
            v["POWERLEVEL10K"],
        )],
        strip_prefix = "powerlevel10k-{}".format(v["POWERLEVEL10K"].lstrip("v")),
        build_file_content = _FILEGROUP_ALL,
    )

    # eza tokyo-night theme
    http_file(
        name = "eza_theme",
        urls = ["https://raw.githubusercontent.com/eza-community/eza-themes/main/themes/tokyonight.yml"],
        downloaded_file_path = "theme.yml",
    )

tool_repos = module_extension(implementation = _tool_repos_impl)
