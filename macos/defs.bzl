"""Static Mach-O archives for the Neovim C libraries.

The hermetic macOS toolchain finds an SDK with xcrun. This rule does not use
that toolchain. zig cc gets the pinned Mac SDK.
"""

# Zig 0.15 instruments C unless the sanitize flag is off.
# Upstream deps are Release, PIC, and C99 with extensions.
_COMMON = [
    "-fno-sanitize=undefined",
    "-fPIC",
    "-O3",
    "-DNDEBUG",
    "-std=gnu99",
]

_TARGET = "aarch64-macos"

# libuv Darwin backend. Linux epoll sources stay out of this list.
_LIBUV_SRCS = [
    "src/fs-poll.c",
    "src/idna.c",
    "src/inet.c",
    "src/random.c",
    "src/strscpy.c",
    "src/strtok.c",
    "src/thread-common.c",
    "src/threadpool.c",
    "src/timer.c",
    "src/uv-common.c",
    "src/uv-data-getter-setters.c",
    "src/version.c",
    "src/unix/async.c",
    "src/unix/core.c",
    "src/unix/dl.c",
    "src/unix/fs.c",
    "src/unix/getaddrinfo.c",
    "src/unix/getnameinfo.c",
    "src/unix/loop-watcher.c",
    "src/unix/loop.c",
    "src/unix/pipe.c",
    "src/unix/poll.c",
    "src/unix/process.c",
    "src/unix/random-devurandom.c",
    "src/unix/signal.c",
    "src/unix/stream.c",
    "src/unix/tcp.c",
    "src/unix/thread.c",
    "src/unix/tty.c",
    "src/unix/udp.c",
    "src/unix/proctitle.c",
    "src/unix/bsd-ifaddrs.c",
    "src/unix/kqueue.c",
    "src/unix/random-getentropy.c",
    "src/unix/darwin-proctitle.c",
    "src/unix/darwin.c",
    "src/unix/fsevents.c",
]

_LIBUV_FLAGS = [
    "-Wall",
    "-Wextra",
    "-Wstrict-prototypes",
    "-Wno-unused-parameter",
    "-fno-strict-aliasing",
]

_LIBUV_DEFS = [
    "_FILE_OFFSET_BITS=64",
    "_LARGEFILE_SOURCE",
    "_DARWIN_UNLIMITED_SELECT=1",
    "_DARWIN_USE_64_BIT_INODE=1",
]

_TS_SRCS = [
    "lib/src/alloc.c",
    "lib/src/get_changed_ranges.c",
    "lib/src/language.c",
    "lib/src/lexer.c",
    "lib/src/node.c",
    "lib/src/parser.c",
    "lib/src/point.c",
    "lib/src/query.c",
    "lib/src/stack.c",
    "lib/src/subtree.c",
    "lib/src/tree.c",
    "lib/src/tree_cursor.c",
    "lib/src/wasm_store.c",
]

_TS_FLAGS = [
    "-std=gnu11",
    "-Wall",
    "-Wextra",
    "-Wshadow",
    "-Wpedantic",
    "-Werror=incompatible-pointer-types",
    "-Werror=strict-aliasing",
    "-Wstrict-aliasing=2",
    "-fvisibility=hidden",
]

def _lib(name, out, tar, srcs, incs, defines = [], cflags = [], extra = {}, patches = None):
    if patches == None:
        patches = []
    return struct(
        name = name,
        out = out,
        tar = tar,
        srcs = [name + ":" + src for src in srcs],
        incs = [_inc(name, inc) for inc in incs],
        defines = defines,
        cflags = cflags,
        extra = dict(extra),
        patches = patches,
    )

def _inc(name, item):
    if ":" in item:
        return item
    return name + ":" + item

# One row per static archive. luv uses LuaJIT headers and does not link LuaJIT.
_LIBS = [
    _lib(
        "libuv",
        "libuv.a",
        "@nvim_libuv//file:archive.tar.gz",
        _LIBUV_SRCS,
        ["include", "src"],
        _LIBUV_DEFS,
        _LIBUV_FLAGS,
    ),
    _lib(
        "luv",
        "libluv.a",
        "@nvim_luv//file:archive.tar.gz",
        ["src/luv.c"],
        ["src", "libuv:include", "luajit:src", "compat:c-api"],
        [],
        ["-w"],
        {
            "@nvim_libuv//file:archive.tar.gz": "libuv",
            "@nvim_luajit//file:archive.tar.gz": "luajit",
            "@nvim_lua_compat53//file:archive.tar.gz": "compat",
        },
    ),
    _lib(
        "lpeg",
        "liblpeg.a",
        "@nvim_lpeg//file:archive.tar.gz",
        ["lpcap.c", "lpcode.c", "lpcset.c", "lpprint.c", "lptree.c", "lpvm.c"],
        [".", "luajit:src"],
        [],
        ["-w"],
        {"@nvim_luajit//file:archive.tar.gz": "luajit"},
    ),
    _lib(
        "unibilium",
        "libunibilium.a",
        "@nvim_unibilium//file:archive.tar.gz",
        ["unibilium.c", "uninames.c", "uniutil.c"],
        ["."],
        # Empty path. unibilium does not open a host terminfo directory.
        ['TERMINFO_DIRS=""'],
        [],
        {},
        ["//third_party/patches:unibilium_no_host.patch"],
    ),
    _lib(
        "utf8proc",
        "libutf8proc.a",
        "@nvim_utf8proc//file:archive.tar.gz",
        ["utf8proc.c"],
        ["."],
        ["UTF8PROC_STATIC", "UTF8PROC_EXPORTS"],
        ["-O2", "-std=c99", "-pedantic", "-Wall"],
    ),
    _lib(
        "tree_sitter",
        "libtree-sitter.a",
        "@nvim_treesitter//file:archive.tar.gz",
        _TS_SRCS,
        ["lib/src", "lib/src/wasm", "lib/include"],
        [],
        _TS_FLAGS,
    ),
]

def _pair(item):
    parts = item.split(":")
    if len(parts) != 2 or parts[0] == "" or parts[1] == "":
        fail("need alias:rel, got " + item)
    return parts[0], parts[1]

def _macho_lib_impl(ctx):
    out = ctx.actions.declare_file(ctx.attr.out)
    recipe = ctx.actions.declare_file(ctx.label.name + ".recipe")
    lines = ["target\t" + ctx.attr.target]
    inputs = [ctx.file.src, ctx.file._sdk]
    lines.append("tar\t%s\t%s" % (ctx.attr.src_name, ctx.file.src.path))
    for dep, alias in ctx.attr.extra_srcs.items():
        files = dep.files.to_list()
        if len(files) != 1:
            fail(alias + " must be one archive")
        lines.append("tar\t%s\t%s" % (alias, files[0].path))
        inputs.append(files[0])
    for patch in ctx.files.patches:
        lines.append("patch\t%s\t%s" % (ctx.attr.src_name, patch.path))
        inputs.append(patch)
    for item in ctx.attr.includes:
        alias, rel = _pair(item)
        lines.append("inc\t%s\t%s" % (alias, rel))
    for item in ctx.attr.defines:
        lines.append("def\t" + item)
    for item in ctx.attr.cflags:
        lines.append("flag\t" + item)
    for item in ctx.attr.sources:
        alias, rel = _pair(item)
        lines.append("unit\t%s\t%s" % (alias, rel))
    ctx.actions.write(recipe, "\n".join(lines) + "\n")
    inputs.append(recipe)
    ctx.actions.run(
        executable = ctx.executable._script,
        arguments = [
            ctx.executable._zig.path,
            ctx.file._sdk.path,
            out.path,
            recipe.path,
            ctx.files._toolbox[0].path,
        ],
        inputs = inputs + ctx.files._zig_files + ctx.files._toolbox,
        tools = [ctx.executable._zig],
        outputs = [out],
        mnemonic = "ZigMachOArchive",
    )
    return [DefaultInfo(files = depset([out]))]

_macho_lib = rule(
    implementation = _macho_lib_impl,
    attrs = {
        "cflags": attr.string_list(),
        "defines": attr.string_list(),
        "extra_srcs": attr.label_keyed_string_dict(allow_files = True),
        "includes": attr.string_list(),
        "out": attr.string(mandatory = True),
        "patches": attr.label_list(allow_files = True),
        "sources": attr.string_list(mandatory = True),
        "src": attr.label(mandatory = True, allow_single_file = True),
        "src_name": attr.string(mandatory = True),
        "target": attr.string(mandatory = True),
        "_script": attr.label(
            default = ":archive.sh",
            executable = True,
            cfg = "exec",
            allow_single_file = True,
        ),
        "_sdk": attr.label(default = "@macos_sdk//:sdk.tar", allow_single_file = True),
        "_zig": attr.label(
            default = "@zig_sdk//:zig",
            executable = True,
            cfg = "exec",
            allow_single_file = True,
        ),
        "_toolbox": attr.label(default = ":exec_toolbox", cfg = "exec"),
        "_zig_files": attr.label(default = "@zig_sdk//:all"),
    },
)

def _shortest_configure(files):
    configure = None
    for src in files:
        if src.basename != "configure":
            continue
        if configure == None or len(src.path) < len(configure.path):
            configure = src
    if configure == None:
        fail("configure is missing")
    return configure

def _exec_make_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name)
    configure = _shortest_configure(ctx.files.src)
    ctx.actions.run(
        executable = ctx.executable._script,
        arguments = [
            out.path,
            ctx.executable._zig.path,
            ctx.file.sdk.path,
            configure.path,
            ctx.files._toolbox[0].path,
        ],
        inputs = ctx.files.src + [ctx.file.sdk] + ctx.files._zig_files + ctx.files._toolbox,
        tools = [ctx.executable._zig],
        outputs = [out],
        mnemonic = "ZigMake",
    )
    return [DefaultInfo(files = depset([out]), executable = out)]

_exec_make = rule(
    implementation = _exec_make_impl,
    executable = True,
    attrs = {
        "sdk": attr.label(mandatory = True, allow_single_file = True),
        "src": attr.label(mandatory = True),
        "_script": attr.label(
            default = ":host_make.sh",
            executable = True,
            cfg = "exec",
            allow_single_file = True,
        ),
        "_toolbox": attr.label(default = ":exec_toolbox", cfg = "exec"),
        "_zig": attr.label(
            default = "@zig_sdk//:zig",
            executable = True,
            cfg = "exec",
            allow_single_file = True,
        ),
        "_zig_files": attr.label(default = "@zig_sdk//:all"),
    },
)

def _exec_toolbox_impl(ctx):
    out = ctx.actions.declare_directory(ctx.label.name)
    anchor = _shortest_configure(ctx.files.src)
    ctx.actions.run(
        executable = ctx.executable._script,
        arguments = [
            out.path,
            ctx.executable._zig.path,
            ctx.file.sdk.path,
            anchor.path,
            ctx.file.mini.path,
        ],
        inputs = ctx.files.src + [ctx.file.sdk, ctx.file.mini] + ctx.files._zig_files,
        tools = [ctx.executable._zig],
        outputs = [out],
        mnemonic = "ZigToolbox",
    )
    return [DefaultInfo(files = depset([out]))]

_exec_toolbox = rule(
    implementation = _exec_toolbox_impl,
    attrs = {
        "mini": attr.label(default = ":toolbox.miniconfig", allow_single_file = True),
        "sdk": attr.label(mandatory = True, allow_single_file = True),
        "src": attr.label(mandatory = True),
        "_script": attr.label(
            default = ":toolbox.sh",
            executable = True,
            cfg = "exec",
            allow_single_file = True,
        ),
        "_zig": attr.label(
            default = "@zig_sdk//:zig",
            executable = True,
            cfg = "exec",
            allow_single_file = True,
        ),
        "_zig_files": attr.label(default = "@zig_sdk//:all"),
    },
)

def _macho_luajit_impl(ctx):
    out = ctx.actions.declare_file(ctx.attr.out)
    # The SDK tar is an action input. The script extracts it.
    ctx.actions.run(
        executable = ctx.executable._script,
        arguments = [
            out.path,
            ctx.executable._zig.path,
            ctx.file.sdk.path,
            ctx.file.src.path,
            ctx.attr.target,
            ctx.executable.make.path,
            ctx.files._toolbox[0].path,
        ],
        inputs = [ctx.file.src, ctx.file.sdk] + ctx.files._zig_files + ctx.files._toolbox,
        tools = [ctx.executable._zig, ctx.executable.make],
        outputs = [out],
        mnemonic = "ZigMachOLuaJIT",
    )
    return [DefaultInfo(files = depset([out]))]

_macho_luajit = rule(
    implementation = _macho_luajit_impl,
    attrs = {
        "make": attr.label(mandatory = True, executable = True, cfg = "exec"),
        "out": attr.string(mandatory = True),
        "sdk": attr.label(mandatory = True, allow_single_file = True),
        "src": attr.label(mandatory = True, allow_single_file = True),
        "target": attr.string(mandatory = True),
        "_toolbox": attr.label(default = ":exec_toolbox", cfg = "exec"),
        "_script": attr.label(
            default = ":luajit.sh",
            executable = True,
            cfg = "exec",
            allow_single_file = True,
        ),
        "_zig": attr.label(
            default = "@zig_sdk//:zig",
            executable = True,
            cfg = "exec",
            allow_single_file = True,
        ),
        "_zig_files": attr.label(default = "@zig_sdk//:all"),
    },
)

def nvim_macos_deps():
    """aarch64-macos static archives. The public group is nvim_deps_aarch64."""
    names = []
    for lib in _LIBS:
        _macho_lib(
            name = lib.name,
            out = lib.out,
            src = lib.tar,
            src_name = lib.name,
            extra_srcs = lib.extra,
            sources = lib.srcs,
            includes = lib.incs,
            defines = lib.defines,
            cflags = _COMMON + lib.cflags,
            patches = lib.patches,
            target = _TARGET,
            visibility = ["//visibility:public"],
        )
        names.append(":" + lib.name)
    native.filegroup(
        name = "nvim_deps_aarch64",
        srcs = names,
        visibility = ["//visibility:public"],
    )
    _exec_toolbox(
        name = "exec_toolbox",
        src = "@toybox_src//:all_srcs",
        sdk = "@macos_sdk//:sdk.tar",
        visibility = ["//third_party/nvim:__pkg__"],
    )
    _exec_make(
        name = "exec_make",
        src = "@make_src//:all_srcs",
        sdk = "@macos_sdk//:sdk.tar",
        visibility = ["//third_party/nvim:__pkg__"],
    )
    for cpu, triple in (("aarch64", "aarch64-macos"), ("x86_64", "x86_64-macos")):
        _macho_luajit(
            name = "luajit_" + cpu,
            out = "libluajit_" + cpu + ".a",
            src = "@nvim_luajit//file:archive.tar.gz",
            sdk = "@macos_sdk//:sdk.tar",
            target = triple,
            make = ":exec_make",
            visibility = ["//visibility:public"],
        )
