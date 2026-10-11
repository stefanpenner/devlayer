"""Zig selects the target CPU. The host does not run the binary."""

# (os, arch) -> (platform, toolchain)
_ZIG = {
    ("linux", "x86_64"): (
        Label("@zig_sdk//platform:linux_amd64"),
        Label("@zig_sdk//toolchain:linux_amd64_musl"),
    ),
    ("linux", "aarch64"): (
        Label("@zig_sdk//platform:linux_arm64"),
        Label("@zig_sdk//toolchain:linux_arm64_musl"),
    ),
    ("windows", "x86_64"): (
        Label("@zig_sdk//platform:windows_amd64"),
        Label("@zig_sdk//toolchain:windows_amd64"),
    ),
    ("windows", "aarch64"): (
        Label("@zig_sdk//platform:windows_arm64"),
        Label("@zig_sdk//toolchain:windows_arm64"),
    ),
}

def _linux_transition_impl(settings, attr):
    pair = _ZIG.get((attr.os, attr.arch))
    if not pair:
        fail("unsupported os/arch: " + attr.os + "/" + attr.arch)
    plat, toolchain = pair
    return {
        "//command_line_option:platforms": [str(plat)],
        "//command_line_option:extra_toolchains": [str(toolchain)],
    }

linux_transition = transition(
    implementation = _linux_transition_impl,
    inputs = [],
    outputs = [
        "//command_line_option:extra_toolchains",
        "//command_line_option:platforms",
    ],
)

def _allowlist():
    return {
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    }

def _bin_file(files, name):
    hits = [f for f in files if f.basename == name]
    if not hits:
        fail("missing binary " + name)
    for f in hits:
        if f.short_path.endswith("/bin/" + name) or f.short_path == "bin/" + name:
            return f
    return hits[0]

def _flat_archive_impl(ctx):
    src = _bin_file(ctx.files.src, ctx.attr.member)
    out = ctx.actions.declare_file(ctx.label.name + ".tar.gz")
    ctx.actions.run(
        executable = ctx.executable._pack,
        arguments = ["file", "--src", src.path, "--name", ctx.attr.member, "--out", out.path],
        inputs = [src],
        outputs = [out],
        mnemonic = "PackFile",
    )
    return DefaultInfo(files = depset([out]))

flat_archive = rule(
    implementation = _flat_archive_impl,
    attrs = {
        "arch": attr.string(mandatory = True),
        "member": attr.string(mandatory = True),
        "os": attr.string(default = "linux"),
        "src": attr.label(mandatory = True, cfg = linux_transition),
        "_pack": attr.label(
            default = "//tools/packarchive",
            executable = True,
            cfg = "exec",
        ),
    } | _allowlist(),
)

def _rel_under(short_path, root):
    prefix = root.rstrip("/") + "/"
    if not short_path.startswith(prefix):
        fail(short_path + " does not start with " + prefix)
    return short_path[len(prefix):]

def _tree_archive_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".tar.gz")
    files = ctx.files.src
    if not files:
        fail("empty install tree")
    lines = []
    for f in files:
        kind = "d" if f.is_directory else "f"
        lines.append(kind + "\t" + f.path + "\t" + _rel_under(f.short_path, ctx.attr.root))
    manifest = ctx.actions.declare_file(ctx.label.name + ".manifest")
    ctx.actions.write(manifest, "\n".join(lines) + "\n")
    ctx.actions.run(
        executable = ctx.executable._pack,
        arguments = ["tree", "--manifest", manifest.path, "--out", out.path],
        inputs = files + [manifest],
        outputs = [out],
        mnemonic = "PackTree",
    )
    return DefaultInfo(files = depset([out]))

tree_archive = rule(
    implementation = _tree_archive_impl,
    attrs = {
        "arch": attr.string(mandatory = True),
        "os": attr.string(default = "linux"),
        "root": attr.string(mandatory = True),
        "src": attr.label(mandatory = True, cfg = linux_transition),
        "_pack": attr.label(
            default = "//tools/packarchive",
            executable = True,
            cfg = "exec",
        ),
    } | _allowlist(),
)
