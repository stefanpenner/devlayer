"""Module extension for fetching all third-party tool sources and pre-built binaries.

Every archive URL must have a sha256 in //internal/sums:checksums.sha256.
No pin → the extension fails closed.
"""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")
load("@versions//:versions.bzl", "VERSIONS")
load("//tools:glow.bzl", "glow_src")

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

# Neovim cmake.deps downloads these unless the URL cache entries point at the files.
# The names match third_party/nvim/defs.bzl.
_NVIM_DEP_URLS = [
    ("nvim_libuv", "https://github.com/libuv/libuv/archive/v1.52.1.tar.gz"),
    ("nvim_luajit", "https://github.com/luajit/luajit/archive/fbb36bb6bfa88716a47c58bcf9ce9f2ef752abac.tar.gz"),
    ("nvim_lua_compat53", "https://github.com/lunarmodules/lua-compat-5.3/archive/v0.13.tar.gz"),
    ("nvim_luv", "https://github.com/luvit/luv/archive/1.52.1-0.tar.gz"),
    ("nvim_lpeg", "https://github.com/neovim/deps/raw/d495ee6f79e7962a53ad79670cb92488abe0b9b4/opt/lpeg-1.1.0.tar.gz"),
    ("nvim_unibilium", "https://github.com/neovim/unibilium/archive/v2.1.2.tar.gz"),
    ("nvim_utf8proc", "https://github.com/juliastrings/utf8proc/archive/v2.11.3.tar.gz"),
    ("nvim_treesitter", "https://github.com/tree-sitter/tree-sitter/archive/v0.26.13.tar.gz"),
    ("nvim_treesitter_c", "https://github.com/tree-sitter/tree-sitter-c/archive/v0.24.1.tar.gz"),
    ("nvim_treesitter_lua", "https://github.com/tree-sitter-grammars/tree-sitter-lua/archive/v0.5.0.tar.gz"),
    ("nvim_treesitter_vim", "https://github.com/tree-sitter-grammars/tree-sitter-vim/archive/v0.8.1.tar.gz"),
    ("nvim_treesitter_vimdoc", "https://github.com/neovim/tree-sitter-vimdoc/archive/v4.1.0.tar.gz"),
    ("nvim_treesitter_query", "https://github.com/tree-sitter-grammars/tree-sitter-query/archive/v0.8.0.tar.gz"),
    ("nvim_treesitter_markdown", "https://github.com/tree-sitter-grammars/tree-sitter-markdown/archive/v0.5.3.tar.gz"),
]

# Install deps into the foreign_cc prefix. Pass the host compiler to LuaJIT.
# Pass the Linux system name into each ExternalProject.
_NVIM_PATCH = """
set -eu
grep -F 'set(DEPS_INSTALL_DIR "${CMAKE_BINARY_DIR}/usr")' cmake/Deps.cmake >/dev/null
awk 'BEGIN{done=0} { if (!done && index($0, "set(DEPS_INSTALL_DIR \\"${CMAKE_BINARY_DIR}/usr\\")")) { print "if(\\"$ENV{NVIM_DEPS_PREFIX}\\" STREQUAL \\"\\")"; print "  set(DEPS_INSTALL_DIR \\"${CMAKE_BINARY_DIR}/usr\\")"; print "else()"; print "  set(DEPS_INSTALL_DIR \\"$ENV{NVIM_DEPS_PREFIX}\\")"; print "endif()"; done=1; next } print }' cmake/Deps.cmake > cmake/Deps.cmake.new
mv cmake/Deps.cmake.new cmake/Deps.cmake
grep -F 'NVIM_DEPS_PREFIX' cmake/Deps.cmake >/dev/null
grep -F -- '-D CMAKE_INSTALL_PREFIX=${DEPS_INSTALL_DIR})' cmake/Deps.cmake >/dev/null
sed -i.bak 's|-D CMAKE_INSTALL_PREFIX=${DEPS_INSTALL_DIR})|-D CMAKE_INSTALL_PREFIX=${DEPS_INSTALL_DIR} -D CMAKE_SYSTEM_NAME=${CMAKE_SYSTEM_NAME} -D CMAKE_SYSTEM_PROCESSOR=${CMAKE_SYSTEM_PROCESSOR} -D CMAKE_TRY_COMPILE_TARGET_TYPE=STATIC_LIBRARY -D CMAKE_RANLIB=$ENV{NVIM_RANLIB})|' cmake/Deps.cmake
grep -F 'CMAKE_RANLIB=$ENV{NVIM_RANLIB}' cmake/Deps.cmake >/dev/null
grep -F 'elseif(UNIX)' cmake.deps/cmake/BuildLuajit.cmake >/dev/null
grep -F 'CC=${DEPS_C_COMPILER} PREFIX=${DEPS_INSTALL_DIR}' cmake.deps/cmake/BuildLuajit.cmake >/dev/null
awk 'BEGIN{inserted=0} { if (!inserted && $0 ~ /elseif\\(UNIX\\)/) { print; print "  get_filename_component(_nvim_zig_bin \\"$ENV{NVIM_ZIG}\\" DIRECTORY)"; inserted=1; next } print }' cmake.deps/cmake/BuildLuajit.cmake > cmake.deps/cmake/BuildLuajit.cmake.new
mv cmake.deps/cmake/BuildLuajit.cmake.new cmake.deps/cmake/BuildLuajit.cmake
sed -i.bak 's|CC=${DEPS_C_COMPILER} PREFIX=${DEPS_INSTALL_DIR}|CC=${DEPS_C_COMPILER} HOST_CC=${NVIM_HOST_CC} TARGET_SYS=Linux BUILDMODE=static TARGET_LDFLAGS=-lunwind "TARGET_AR=${_nvim_zig_bin}/tools/ar rcus" PREFIX=${DEPS_INSTALL_DIR}|' cmake.deps/cmake/BuildLuajit.cmake
grep -F 'find_path2(ICONV_INCLUDE_DIR NAMES iconv.h)' cmake/FindIconv.cmake >/dev/null
awk 'BEGIN{inserted=0} { print; if (!inserted && $0 ~ /find_path2\\(ICONV_INCLUDE_DIR NAMES iconv.h\\)/) { print "if(NOT ICONV_INCLUDE_DIR)"; print "  get_filename_component(_cc_dir \\"${CMAKE_C_COMPILER}\\" DIRECTORY)"; print "  get_filename_component(_cc_tools \\"${_cc_dir}\\" DIRECTORY)"; print "  get_filename_component(_zig_root \\"${_cc_tools}\\" DIRECTORY)"; print "  set(_musl_inc \\"${_zig_root}/lib/libc/include/generic-musl\\")"; print "  if(EXISTS \\"${_musl_inc}/iconv.h\\")"; print "    set(ICONV_INCLUDE_DIR \\"${_musl_inc}\\")"; print "  endif()"; print "endif()"; inserted=1 } }' cmake/FindIconv.cmake > cmake/FindIconv.cmake.new
mv cmake/FindIconv.cmake.new cmake/FindIconv.cmake
grep -F 'include/generic-musl' cmake/FindIconv.cmake >/dev/null
"""

def _exports(names):
    return 'exports_files([{}], visibility = ["//visibility:public"])'.format(
        ", ".join(['"%s"' % n for n in names]),
    )

def _parse_sums(text):
    sums = {}
    for raw in text.split("\n"):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split("  ")
        if len(parts) != 2:
            fail("bad checksum line: " + line)
        sha = parts[0].strip()
        url = parts[1].strip()
        if len(sha) != 64:
            fail("bad sha256 for " + url)
        if url in sums:
            fail("duplicate pin for " + url)
        sums[url] = sha
    return sums

def _read_sums(module_ctx):
    label = Label("//internal/sums:checksums.sha256")
    module_ctx.watch(label)
    return _parse_sums(module_ctx.read(label))

def _pin(sums, urls):
    # Pin is the first URL. Later URLs are byte-identical fallbacks (Bazel
    # checks this sha256 against whichever mirror answers).
    if not urls:
        fail("empty url list")
    url = urls[0]
    got = sums.get(url)
    if not got:
        fail("no sha256 pin for " + url)
    return got

def _http_archive(sums, **kwargs):
    kwargs["sha256"] = _pin(sums, kwargs["urls"])
    http_archive(**kwargs)

def _http_file(sums, **kwargs):
    kwargs["sha256"] = _pin(sums, kwargs["urls"])
    http_file(**kwargs)

def _tool_repos_impl(module_ctx):
    sums = _read_sums(module_ctx)
    v = VERSIONS

    # ═══════════════════════════════════════════════════════════════════════════
    # Source builds (compiled via rules_foreign_cc)
    # ═══════════════════════════════════════════════════════════════════════════

    _http_archive(sums, 
        name = "btop_src",
        urls = ["https://github.com/aristocratos/btop/archive/refs/tags/v{}.tar.gz".format(v["BTOP"])],
        strip_prefix = "btop-{}".format(v["BTOP"]),
        build_file_content = _ALL_SRCS,
        patch_cmds = [
            "sed -i.bak 's/cmake_minimum_required(VERSION 3.25)/cmake_minimum_required(VERSION 3.20)/' CMakeLists.txt",
            """sed -i.bak 's/\\$<LINK_LIBRARY:FRAMEWORK,CoreFoundation>/"-framework CoreFoundation"/g; s/\\$<LINK_LIBRARY:FRAMEWORK,IOKit>/"-framework IOKit"/g' CMakeLists.txt""",
            "sed -i.bak '/add_subdirectory.*tests/d' CMakeLists.txt",
            "sed -i.bak 's/--fatal-warnings//g' CMakeLists.txt",
            # CMake 3.25 defines LINUX. The pinned CMake is 3.23.
            """sed -i.bak 's/elseif(LINUX)/elseif(CMAKE_SYSTEM_NAME STREQUAL "Linux")/; s/if(LINUX AND BTOP_GPU)/if(CMAKE_SYSTEM_NAME STREQUAL "Linux" AND BTOP_GPU)/' CMakeLists.txt""",
            # This libc++ cannot form a constexpr optional of filesystem::path.
            "sed -i.bak 's/static constexpr auto get_xdg_state_dir/static auto get_xdg_state_dir/' src/btop_config.cpp",
            # ifstream is not a constant expression. Drop constexpr on the CPU probe.
            "sed -i.bak 's/static constexpr auto detect_active_cpus/static auto detect_active_cpus/' src/linux/btop_collect.cpp",
            # void unexpected() lives in namespace std. The class lives in std::__1.
            # std::unexpected<int> is still ambiguous. Qualify the class.
            "sed -i.bak 's/std::unexpected {/std::__1::unexpected<int> {/g' src/btop_cli.cpp",
        ],
    )

    _http_archive(sums, 
        name = "htop_src",
        urls = ["https://github.com/htop-dev/htop/releases/download/{0}/htop-{0}.tar.xz".format(v["HTOP"])],
        strip_prefix = "htop-{}".format(v["HTOP"]),
        build_file_content = _ALL_SRCS,
    )

    _http_archive(sums, 
        name = "make_src",
        urls = ["https://ftp.gnu.org/gnu/make/make-{}.tar.gz".format(v["MAKE"])],
        strip_prefix = "make-{}".format(v["MAKE"]),
        build_file_content = _ALL_SRCS,
    )

    _http_archive(sums,
        name = "toybox_src",
        urls = ["https://github.com/landley/toybox/archive/refs/tags/{}.tar.gz".format(v["TOYBOX"])],
        strip_prefix = "toybox-{}".format(v["TOYBOX"]),
        build_file_content = _ALL_SRCS,
        patches = ["//third_party/patches:toybox_apple_nproc.patch"],
        patch_args = ["-p1"],
    )

    _http_archive(sums,
        name = "libiconv_src",
        urls = ["https://ftp.gnu.org/gnu/libiconv/libiconv-{}.tar.gz".format(v["ICONV"])],
        strip_prefix = "libiconv-{}".format(v["ICONV"]),
        build_file_content = """\
filegroup(
    name = "all",
    srcs = glob(["**"]),
    visibility = ["//visibility:public"],
)
exports_files(["README"], visibility = ["//visibility:public"])
""",
    )

    _http_archive(sums, 
        name = "zlib_src",
        urls = ["https://zlib.net/fossils/zlib-{}.tar.gz".format(v["ZLIB"])],
        strip_prefix = "zlib-{}".format(v["ZLIB"]),
        build_file_content = _ALL_SRCS,
    )

    _http_archive(sums, 
        name = "ncurses_src",
        urls = ["https://ftp.gnu.org/gnu/ncurses/ncurses-{}.tar.gz".format(v["NCURSES"])],
        strip_prefix = "ncurses-{}".format(v["NCURSES"]),
        build_file_content = _ALL_SRCS,
    )

    _http_archive(sums, 
        name = "git_src",
        urls = ["https://mirrors.edge.kernel.org/pub/software/scm/git/git-{}.tar.xz".format(v["GIT"])],
        strip_prefix = "git-{}".format(v["GIT"]),
        build_file_content = _ALL_SRCS,
    )

    _http_archive(sums, 
        name = "zsh_src",
        # Current tarball is /pub. /pub/old is the same bytes after zsh.org moves it.
        # The pin is the /pub URL. /old 404s until that move.
        urls = [
            "https://www.zsh.org/pub/zsh-{}.tar.xz".format(v["ZSH"]),
            "https://www.zsh.org/pub/old/zsh-{}.tar.xz".format(v["ZSH"]),
        ],
        strip_prefix = "zsh-{}".format(v["ZSH"]),
        build_file_content = _ALL_SRCS,
        # configure writes config.modules. This wrapper then sets link=static.
        # The list is the static zsh modules.
        patch_cmds = ["""cat > devlayer-configure << 'END'
#!/bin/sh
set -eu
cd "$(dirname "$0")"
./configure "$@"
awk '
BEGIN {
  n = split("compctl complete complist computil zle zutil parameter terminfo datetime stat system mathfunc files regex zpty mapfile langinfo net/socket net/tcp", mods, " ")
}
{
  line = $0
  for (i = 1; i <= n; i++) {
    needle = "name=zsh/" mods[i]
    p = index(line, needle)
    if (p == 0) continue
    c = substr(line, p + length(needle), 1)
    if (c == "" || c == " " || c == "\\t") {
      gsub("link=no", "link=static", line)
      gsub("load=no", "load=yes", line)
      break
    }
  }
  print line
}
' config.modules > config.modules.devlayer
mv config.modules.devlayer config.modules
END
chmod 755 devlayer-configure
"""],
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
        _http_archive(sums, 
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
        _http_archive(sums, 
            name = "bat_" + plat,
            urls = ["https://github.com/sharkdp/bat/releases/download/v{ver}/{name}.tar.gz".format(
                ver = v["BAT"],
                name = name,
            )],
            strip_prefix = name,
            build_file_content = _exports(["bat"]),
        )

    # ripgrep — all three platforms ship musl/darwin builds
    for plat, target in [
        ("linux_amd64", "x86_64-unknown-linux-musl"),
        ("linux_arm64", "aarch64-unknown-linux-musl"),
        ("darwin_arm64", "aarch64-apple-darwin"),
    ]:
        name = "ripgrep-{ver}-{t}".format(ver = v["RG"], t = target)
        _http_archive(sums, 
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
        _http_archive(sums, 
            name = "delta_" + plat,
            urls = ["https://github.com/dandavison/delta/releases/download/{ver}/{name}.tar.gz".format(
                ver = v["DELTA"],
                name = name,
            )],
            strip_prefix = name,
            build_file_content = _exports(["delta"]),
        )

    # dust
    for plat, target in [
        ("linux_amd64", "x86_64-unknown-linux-musl"),
        ("linux_arm64", "aarch64-unknown-linux-musl"),
        ("darwin_arm64", "aarch64-apple-darwin"),
    ]:
        name = "dust-v{ver}-{t}".format(ver = v["DUST"], t = target)
        _http_archive(sums, 
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
        _http_archive(sums, 
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
        _http_archive(sums, 
            name = "lazygit_" + plat,
            urls = ["https://github.com/jesseduffield/lazygit/releases/download/v{ver}/lazygit_{ver}_{os}_{arch}.tar.gz".format(
                ver = v["LAZYGIT"],
                os = lazygit_os,
                arch = arch,
            )],
            build_file_content = _exports(["lazygit"]),
        )

    # glow — compiled from the tag archive. See //third_party/glow.
    glow_url = "https://github.com/charmbracelet/glow/archive/refs/tags/v{}.tar.gz".format(v["GLOW"])
    go_ver = v["GO"]
    go_urls = {
        "darwin_arm64": "https://go.dev/dl/go{}.darwin-arm64.tar.gz".format(go_ver),
        "linux_amd64": "https://go.dev/dl/go{}.linux-amd64.tar.gz".format(go_ver),
        "linux_arm64": "https://go.dev/dl/go{}.linux-arm64.tar.gz".format(go_ver),
    }
    glow_src(
        name = "glow_src",
        url = glow_url,
        sha256 = _pin(sums, [glow_url]),
        strip_prefix = "glow-{}".format(v["GLOW"]),
        version = v["GLOW"],
        go_urls = go_urls,
        go_sha256 = {key: _pin(sums, [url]) for key, url in go_urls.items()},
    )

    # age — contains age/ directory with age + age-keygen
    for plat, os_name, goarch in [
        ("linux_amd64", "linux", "amd64"),
        ("linux_arm64", "linux", "arm64"),
        ("darwin_arm64", "darwin", "arm64"),
    ]:
        _http_archive(sums, 
            name = "age_" + plat,
            urls = ["https://github.com/FiloSottile/age/releases/download/v{ver}/age-v{ver}-{os}-{arch}.tar.gz".format(
                ver = v["AGE"],
                os = os_name,
                arch = goarch,
            )],
            strip_prefix = "age",
            build_file_content = _exports(["age", "age-keygen"]),
        )

    # shh — binary at tar root
    for plat, os_name, goarch in [
        ("linux_amd64", "linux", "amd64"),
        ("linux_arm64", "linux", "arm64"),
        ("darwin_arm64", "darwin", "arm64"),
    ]:
        _http_archive(sums, 
            name = "shh_" + plat,
            urls = ["https://github.com/stefanpenner/shh/releases/download/v{ver}/shh_{os}_{arch}.tar.gz".format(
                ver = v["SHH"],
                os = os_name,
                arch = goarch,
            )],
            build_file_content = _exports(["shh"]),
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
        _http_file(sums, 
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
        _http_file(sums, 
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

    # Neovim source. Dep URLs match cmake.deps/deps.txt for this NVIM version.
    _http_archive(sums,
        name = "nvim_src",
        urls = ["https://github.com/neovim/neovim/archive/refs/tags/v{}.tar.gz".format(v["NVIM"])],
        strip_prefix = "neovim-{}".format(v["NVIM"]),
        build_file_content = _ALL_SRCS,
        patches = ["//third_party/patches:nvim_unibilium_superstatic.patch"],
        patch_args = ["-p1"],
        patch_cmds = [_NVIM_PATCH],
    )
    for name, url in _NVIM_DEP_URLS:
        _http_file(sums,
            name = name,
            urls = [url],
            downloaded_file_path = "archive.tar.gz",
        )

    # Neovim pre-built
    for plat, nvim_os, arch in [
        ("linux_amd64", "linux", "x86_64"),
        ("linux_arm64", "linux", "arm64"),
        ("darwin_arm64", "macos", "arm64"),
    ]:
        archive = "nvim-{os}-{arch}".format(os = nvim_os, arch = arch)
        _http_archive(sums, 
            name = "nvim_" + plat,
            urls = ["https://github.com/neovim/neovim/releases/download/v{ver}/{archive}.tar.gz".format(
                ver = v["NVIM"],
                archive = archive,
            )],
            strip_prefix = archive,
            build_file_content = """\
filegroup(
    name = "all",
    srcs = glob(["**"]),
    visibility = ["//visibility:public"],
)
exports_files(
    ["bin/nvim"],
    visibility = ["//visibility:public"],
)
""",
        )

    # Go SDK
    for plat, os_name, goarch in [
        ("linux_amd64", "linux", "amd64"),
        ("linux_arm64", "linux", "arm64"),
        ("darwin_arm64", "darwin", "arm64"),
    ]:
        _http_archive(sums, 
            name = "go_sdk_" + plat,
            urls = ["https://go.dev/dl/go{ver}.{os}-{arch}.tar.gz".format(
                ver = v["GO"],
                os = os_name,
                arch = goarch,
            )],
            strip_prefix = "go",
            build_file_content = _FILEGROUP_ALL + _exports(["bin/go"]),
        )

    # Zig compiler
    for plat, arch, zig_os in [
        ("linux_amd64", "x86_64", "linux"),
        ("linux_arm64", "aarch64", "linux"),
        ("darwin_arm64", "aarch64", "macos"),
    ]:
        prefix = "zig-{arch}-{os}-{ver}".format(arch = arch, os = zig_os, ver = v["ZIG"])
        _http_archive(sums, 
            name = "zig_" + plat,
            urls = ["https://ziglang.org/download/{ver}/{prefix}.tar.xz".format(
                ver = v["ZIG"],
                prefix = prefix,
            )],
            strip_prefix = prefix,
            build_file_content = _FILEGROUP_ALL + _exports(["zig"]),
        )

    # ═══════════════════════════════════════════════════════════════════════════
    # Shell assets (plugins, themes, scripts)
    # ═══════════════════════════════════════════════════════════════════════════

    # bat-extras (batman shell script)
    _http_archive(sums, 
        name = "bat_extras",
        urls = ["https://github.com/eth-p/bat-extras/releases/download/v{ver}/bat-extras-{ver}.zip".format(
            ver = v["BAT_EXTRAS"],
        )],
        build_file_content = _FILEGROUP_ALL + _exports(["bin/batman"]),
    )

    # fzf shell integration (key-bindings.zsh, completion.zsh)
    _http_archive(sums, 
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
""" + _exports(["shell/key-bindings.zsh", "shell/completion.zsh"]),
    )

    # Zsh plugins
    _http_archive(sums, 
        name = "zsh_autosuggestions",
        urls = ["https://github.com/zsh-users/zsh-autosuggestions/archive/refs/tags/{}.tar.gz".format(
            v["ZSH_AUTOSUGGESTIONS"],
        )],
        strip_prefix = "zsh-autosuggestions-{}".format(v["ZSH_AUTOSUGGESTIONS"].lstrip("v")),
        build_file_content = _FILEGROUP_ALL + _exports(["zsh-autosuggestions.zsh"]),
    )

    _http_archive(sums, 
        name = "fast_syntax_highlighting",
        urls = ["https://github.com/zdharma-continuum/fast-syntax-highlighting/archive/refs/tags/{}.tar.gz".format(
            v["FAST_SYNTAX_HIGHLIGHTING"],
        )],
        strip_prefix = "fast-syntax-highlighting-{}".format(v["FAST_SYNTAX_HIGHLIGHTING"].lstrip("v")),
        build_file_content = _FILEGROUP_ALL + _exports(["fast-syntax-highlighting.plugin.zsh"]),
    )

    _http_archive(sums, 
        name = "zsh_history_substring_search",
        urls = ["https://github.com/zsh-users/zsh-history-substring-search/archive/refs/tags/{}.tar.gz".format(
            v["ZSH_HISTORY_SUBSTRING_SEARCH"],
        )],
        strip_prefix = "zsh-history-substring-search-{}".format(v["ZSH_HISTORY_SUBSTRING_SEARCH"].lstrip("v")),
        build_file_content = _FILEGROUP_ALL + _exports(["zsh-history-substring-search.zsh"]),
    )

    _http_archive(sums, 
        name = "powerlevel10k",
        urls = ["https://github.com/romkatv/powerlevel10k/archive/refs/tags/{}.tar.gz".format(
            v["POWERLEVEL10K"],
        )],
        strip_prefix = "powerlevel10k-{}".format(v["POWERLEVEL10K"].lstrip("v")),
        build_file_content = _FILEGROUP_ALL + _exports(["powerlevel10k.zsh-theme"]),
    )

    # romkatv/gitstatus v1.5.4. Matches install.info in powerlevel10k 1.20.0.
    for arch in ["aarch64", "x86_64"]:
        _http_file(sums,
            name = "gitstatusd_linux_" + arch,
            urls = ["https://github.com/romkatv/gitstatus/releases/download/v1.5.4/gitstatusd-linux-%s.tar.gz" % arch],
            downloaded_file_path = "gitstatusd.tar.gz",
        )

tool_repos = module_extension(implementation = _tool_repos_impl)
