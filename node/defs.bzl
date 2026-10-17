"""Node rules: the OCI layout and the VM test target.

node_vm_test builds one binary for Linux x86_64 musl with zig, puts it and its
runfiles in an EROFS test image, and boots the node kernel in the pinned QEMU
with that image as a virtio disk. Init runs the binary as its payload.
Stdout, stderr, and the exit status come back to `bazel test`.

The node boots from the runtime layer alone. `layers` attaches more EROFS
layers as extra read-only disks, for example {"//layers:dev": "dev"}. Init
mounts each one at /layers/<name> after checking its sha256.
"""

load("//linux:cc.bzl", "linux_transition")

def _allowlist():
    return {
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    }

def _runfile_dest(f):
    # _main/<path> for this repo, <repo>/<path> for an external repo.
    p = f.short_path
    if p.startswith("../"):
        return "runfiles/" + p[3:]
    return "runfiles/_main/" + p

def _node_vm_test_impl(ctx):
    binary = ctx.attr.binary[0]
    exe = binary[DefaultInfo].files_to_run.executable
    entries = [{"dest": "entry", "src": exe.path}]
    files = [exe]
    for f in binary[DefaultInfo].default_runfiles.files.to_list():
        entries.append({"dest": _runfile_dest(f), "src": f.path})
        files.append(f)
    manifest = ctx.actions.declare_file(ctx.label.name + ".files.json")
    ctx.actions.write(manifest, json.encode(entries))

    disk = ctx.actions.declare_file(ctx.label.name + ".erofs")
    tools = ctx.attr._hosttools[DefaultInfo].files
    ctx.actions.run(
        executable = ctx.executable._mkimage,
        arguments = [
            "-out",
            disk.path,
            "-mkfs",
            ctx.file._mkfs.path,
            "-loader",
            ctx.file._loader.path,
            "-libpath",
            ctx.file._loader.dirname + ":" + ctx.file._mkfs.dirname + "/../lib",
            "-files",
            manifest.path,
            "-z",
            "lz4",
        ],
        inputs = depset([manifest, ctx.file._mkfs, ctx.file._loader] + files, transitive = [tools]),
        outputs = [disk],
        mnemonic = "NodeTestImage",
    )

    runner = ctx.attr._vmrun[DefaultInfo].files_to_run.executable
    out = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(output = out, target_file = runner, is_executable = True)

    qemu = ctx.file._qemu
    layers = []
    layer_files = []
    for target, name in ctx.attr.layers.items():
        f = target.files.to_list()[0]
        layers.append(name + "=" + f.short_path)
        layer_files.append(f)
    runfiles = ctx.runfiles(
        files = [disk, ctx.file.kernel, ctx.file.initrd, qemu] + layer_files + ctx.files._qemu_files,
    ).merge(ctx.attr._vmrun[DefaultInfo].default_runfiles)
    return [
        DefaultInfo(executable = out, runfiles = runfiles),
        RunEnvironmentInfo(environment = {
            "NODE_QEMU": qemu.short_path,
            "NODE_KERNEL": ctx.file.kernel.short_path,
            "NODE_INITRD": ctx.file.initrd.short_path,
            "NODE_DISK": disk.short_path,
            "NODE_LAYERS": ",".join(layers),
        }),
    ]

node_vm_test = rule(
    implementation = _node_vm_test_impl,
    test = True,
    attrs = {
        "binary": attr.label(mandatory = True, executable = True, cfg = linux_transition),
        "layers": attr.label_keyed_string_dict(allow_files = [".erofs"]),
        "arch": attr.string(default = "x86_64"),
        "os": attr.string(default = "linux"),
        "kernel": attr.label(default = "//node:vmlinuz", allow_single_file = True),
        "initrd": attr.label(default = "//node:initrd.img", allow_single_file = True),
        "_mkimage": attr.label(default = "//node/tools/mkimage", executable = True, cfg = "exec"),
        "_vmrun": attr.label(default = "//node/vmrun", executable = True, cfg = "target"),
        "_mkfs": attr.label(default = "@node_hosttools_linux_x86_64//:usr/bin/mkfs.erofs", allow_single_file = True, cfg = "exec"),
        "_loader": attr.label(default = "@node_hosttools_linux_x86_64//:lib/ld-musl-x86_64.so.1", allow_single_file = True, cfg = "exec"),
        "_hosttools": attr.label(default = "@node_hosttools_linux_x86_64//:all", cfg = "exec"),
        "_qemu": attr.label(default = "//node:qemu-system-x86_64", allow_single_file = True),
        "_qemu_files": attr.label(default = "//node:qemu"),
    } | _allowlist(),
)

def _oci_layout_impl(ctx):
    out = ctx.actions.declare_directory(ctx.label.name)
    args = ["-out", out.path, "-cmdline", ctx.attr.cmdline, "-version", ctx.attr.version]
    inputs = []
    for target, title in ctx.attr.layers.items():
        f = target.files.to_list()[0]
        args += ["-layer", title + "=" + f.path]
        inputs.append(f)
    ctx.actions.run(
        executable = ctx.executable._tool,
        arguments = args,
        inputs = inputs,
        outputs = [out],
        mnemonic = "OciLayout",
    )
    return DefaultInfo(files = depset([out]))

oci_layout = rule(
    implementation = _oci_layout_impl,
    attrs = {
        "layers": attr.label_keyed_string_dict(allow_files = True, mandatory = True),
        "cmdline": attr.string(),
        "version": attr.string(),
        "_tool": attr.label(default = "//node/tools/ocilayout", executable = True, cfg = "exec"),
    },
)
