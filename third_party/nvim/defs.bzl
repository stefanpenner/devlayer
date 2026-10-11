"""Linux nvim. Zig cross-compiles. Host LuaJIT runs the generator."""

load("@rules_foreign_cc//foreign_cc:defs.bzl", "cmake")

# CMake variable, http_file label. Names match tools/extensions.bzl.
_DEPS = [
    ("LIBUV_URL", "@nvim_libuv//file:archive.tar.gz"),
    ("LUAJIT_URL", "@nvim_luajit//file:archive.tar.gz"),
    ("LUA_COMPAT53_URL", "@nvim_lua_compat53//file:archive.tar.gz"),
    ("LUV_URL", "@nvim_luv//file:archive.tar.gz"),
    ("LPEG_URL", "@nvim_lpeg//file:archive.tar.gz"),
    ("UNIBILIUM_URL", "@nvim_unibilium//file:archive.tar.gz"),
    ("UTF8PROC_URL", "@nvim_utf8proc//file:archive.tar.gz"),
    ("TREESITTER_URL", "@nvim_treesitter//file:archive.tar.gz"),
    ("TREESITTER_C_URL", "@nvim_treesitter_c//file:archive.tar.gz"),
    ("TREESITTER_LUA_URL", "@nvim_treesitter_lua//file:archive.tar.gz"),
    ("TREESITTER_VIM_URL", "@nvim_treesitter_vim//file:archive.tar.gz"),
    ("TREESITTER_VIMDOC_URL", "@nvim_treesitter_vimdoc//file:archive.tar.gz"),
    ("TREESITTER_QUERY_URL", "@nvim_treesitter_query//file:archive.tar.gz"),
    ("TREESITTER_MARKDOWN_URL", "@nvim_treesitter_markdown//file:archive.tar.gz"),
]

def _file_url(label):
    # foreign_cc turns $(execpath) into $EXT_BUILD_ROOT/<path> before cmake runs.
    return "file://$(execpath {})".format(label)

def _host_luajit_impl(ctx):
    out = ctx.actions.declare_directory(ctx.label.name)
    ctx.actions.run(
        executable = ctx.executable._script,
        arguments = [
            out.path,
            ctx.executable._zig.path,
            ctx.file.src.path,
            ctx.executable._host_cc.path,
            ctx.executable._make.path,
            ctx.files._toolbox[0].path,
        ],
        inputs = [ctx.file.src, ctx.file._sdk] + ctx.files._zig_files + ctx.files._toolbox,
        tools = [ctx.executable._zig, ctx.executable._host_cc, ctx.executable._make],
        outputs = [out],
        env = {"DEVLAYER_SDKROOT": ctx.file._sdk.path},
        mnemonic = "HostLuaJIT",
    )
    return [DefaultInfo(files = depset([out]))]

_host_luajit = rule(
    implementation = _host_luajit_impl,
    attrs = {
        "src": attr.label(mandatory = True, allow_single_file = True),
        "_host_cc": attr.label(default = ":host_cc.sh", executable = True, cfg = "exec", allow_single_file = True),
        "_script": attr.label(default = ":host_luajit.sh", executable = True, cfg = "exec", allow_single_file = True),
        "_make": attr.label(default = "//macos:exec_make", executable = True, cfg = "exec"),
        "_toolbox": attr.label(default = "//macos:exec_toolbox", cfg = "exec"),
        "_sdk": attr.label(default = "@macos_sdk//:sdk.tar", allow_single_file = True),
        "_zig": attr.label(default = "@zig_sdk//:zig", executable = True, cfg = "exec", allow_single_file = True),
        "_zig_files": attr.label(default = "@zig_sdk//:all"),
    },
)

def _host_nlua0_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".so")
    luajit = ctx.files.luajit[0]
    nlua = None
    for src in ctx.files.nvim_srcs:
        if src.basename == "nlua0.c":
            nlua = src
            break
    if nlua == None:
        fail("nlua0.c is missing from nvim sources")
    ctx.actions.run(
        executable = ctx.executable._script,
        arguments = [
            out.path,
            ctx.executable._zig.path,
            luajit.path,
            ctx.file.lpeg.path,
            ctx.executable._host_cc.path,
            nlua.path,
            ctx.files._toolbox[0].path,
        ],
        inputs = ctx.files.nvim_srcs + [ctx.file.lpeg, luajit, ctx.file._sdk] + ctx.files._zig_files + ctx.files._toolbox,
        tools = [ctx.executable._zig, ctx.executable._host_cc],
        outputs = [out],
        env = {"DEVLAYER_SDKROOT": ctx.file._sdk.path},
        mnemonic = "HostNlua0",
    )
    return [DefaultInfo(files = depset([out]))]

_host_nlua0 = rule(
    implementation = _host_nlua0_impl,
    attrs = {
        "lpeg": attr.label(mandatory = True, allow_single_file = True),
        "luajit": attr.label(mandatory = True),
        "nvim_srcs": attr.label(mandatory = True),
        "_host_cc": attr.label(default = ":host_cc.sh", executable = True, cfg = "exec", allow_single_file = True),
        "_script": attr.label(default = ":host_nlua0.sh", executable = True, cfg = "exec", allow_single_file = True),
        "_sdk": attr.label(default = "@macos_sdk//:sdk.tar", allow_single_file = True),
        "_toolbox": attr.label(default = "//macos:exec_toolbox", cfg = "exec"),
        "_zig": attr.label(default = "@zig_sdk//:zig", executable = True, cfg = "exec", allow_single_file = True),
        "_zig_files": attr.label(default = "@zig_sdk//:all"),
    },
)

# $$$$ is foreign_cc's escape. The build script sees $$VAR$$.
_DEPS_ENV = {
    # Darwin HOSTCC reads this tar. Linux hostcc ignores it.
    "DEVLAYER_SDKROOT": "$(execpath @macos_sdk//:sdk.tar)",
    "NVIM_DEPS_PREFIX": "$$$$INSTALLDIR$$$$",
    "NVIM_RANLIB": "$(execpath :zig_ranlib.sh)",
    "NVIM_ZIG": "$(execpath @zig_sdk//:zig)",
    "ZIG_GLOBAL_CACHE_DIR": "$$$$BUILD_TMPDIR$$$$/zig-cache",
    "ZIG_LOCAL_CACHE_DIR": "$$$$BUILD_TMPDIR$$$$/zig-cache",
}

# STATIC_LIBRARY try_compile reports libraries musl does not have.
_NVIM_CACHE = {
    "CMAKE_BUILD_TYPE": "Release",
    # Host LuaJIT is glibc. Target nlua0 is musl. Same OS is not a cross build.
    "CMAKE_CROSSCOMPILING": "TRUE",
    "CMAKE_SYSTEM_NAME": "Linux",
    "CMAKE_EXE_LINKER_FLAGS": "-static -Wl,--export-dynamic -lunwind",
    "CMAKE_TRY_COMPILE_TARGET_TYPE": "STATIC_LIBRARY",
    "DEPS_PREFIX": "$$$$EXT_BUILD_DEPS$$$$/nvim_deps",
    "ENABLE_LIBINTL": "OFF",
    "ENABLE_LTO": "OFF",
    "HAVE_LIBKSTAT": "0",
    "HAVE_LIBKVM": "0",
    "HAVE_LIBNSL": "0",
    "HAVE_LIBPERFSTAT": "0",
    "HAVE_LIBSENDFILE": "0",
    "LUA_GEN_PRG": "$(execpath :host_luajit)/bin/luajit",
    "LUA_PRG": "$(execpath :host_luajit)/bin/luajit",
    "NLUA0_HOST_PRG": "$(execpath :host_nlua0)",
    "NVIM_HOST_PRG": "$(execpath :host_nvim.sh)",
}

# bin/nvim is the execpath. :all keeps the host runtime beside it.
_HOST_NVIM = [
    ":host_luajit",
    ":host_nlua0",
    ":host_nvim.sh",
    "@nvim_darwin_arm64//:bin/nvim",
    "@nvim_linux_amd64//:bin/nvim",
    "@nvim_linux_arm64//:bin/nvim",
    "@nvim_darwin_arm64//:all",
    "@nvim_linux_amd64//:all",
    "@nvim_linux_arm64//:all",
]

_NVIM_ENV = {
    "DYLD_LIBRARY_PATH": "$(execpath :host_luajit)/lib",
    "LD_LIBRARY_PATH": "$(execpath :host_luajit)/lib",
    "NVIM_HOST_DARWIN": "$(execpath @nvim_darwin_arm64//:bin/nvim)",
    "NVIM_HOST_LINUX": "$(execpath @nvim_linux_amd64//:bin/nvim)",
    "NVIM_HOST_LINUX_ARM64": "$(execpath @nvim_linux_arm64//:bin/nvim)",
    # foreign_cc replaces PATH. touch lives in /bin.
    "PATH": "$(execpath :host_luajit)/bin:/bin:/usr/bin:/usr/local/bin",
}

# Install copies sandbox symlinks. A tree artifact rejects them.
_COPY_LINKS = """find $$INSTALLDIR$$ -type l -print0 | while IFS= read -r -d '' link; do
  target=$(readlink "$link")
  rm "$link"
  cp -p "$target" "$link"
done"""

def _dep_cache():
    cache = {
        "CMAKE_BUILD_TYPE": "Release",
        "CMAKE_TRY_COMPILE_TARGET_TYPE": "STATIC_LIBRARY",
        "NVIM_HOST_CC": "$(execpath :host_cc.sh)",
    }
    for key, label in _DEPS:
        cache[key] = _file_url(label)
    return cache

def nvim_musl():
    """Static musl nvim. //linux:nvim_* selects the CPU."""
    _host_luajit(
        name = "host_luajit",
        src = "@nvim_luajit//file:archive.tar.gz",
    )
    _host_nlua0(
        name = "host_nlua0",
        lpeg = "@nvim_lpeg//file:archive.tar.gz",
        luajit = ":host_luajit",
        nvim_srcs = "@nvim_src//:all_srcs",
    )
    cmake(
        name = "nvim_deps",
        build_data = [label for _, label in _DEPS] + [
            ":host_cc.sh",
            ":zig_ranlib.sh",
            "@macos_sdk//:sdk.tar",
            "@zig_sdk//:zig",
        ],
        cache_entries = _dep_cache(),
        env = _DEPS_ENV,
        install = False,
        # minilua links with Xcode ld. Zig 0.15 panics on LuaJIT eh_frame.
        tags = ["no-sandbox"],
        lib_source = "@nvim_src//:all_srcs",
        # A default libnvim_deps.a would sit inside lib/.
        out_data_dirs = ["lib", "share"],
        out_headers_only = True,
        visibility = ["//visibility:private"],
        working_directory = "cmake.deps",
    )
    cmake(
        name = "nvim_musl",
        build_data = _HOST_NVIM,
        cache_entries = _NVIM_CACHE,
        deps = [":nvim_deps"],
        env = _NVIM_ENV,
        lib_source = "@nvim_src//:all_srcs",
        out_binaries = ["nvim"],
        out_data_dirs = ["lib", "share"],
        postfix_script = _COPY_LINKS,
        visibility = ["//visibility:public"],
    )
