"""Linux bundle build macros — per-tool Docker compilation + assembly."""

def _docker(arch):
    docker_arch = "amd64" if arch == "x86_64" else "arm64"
    return "linux/" + docker_arch, "devlayer-build-base-" + docker_arch

def base_image(arch):
    """Create a genrule that builds the Docker base image for source compilation."""
    docker_platform, tag = _docker(arch)
    # compile_tool looks this tag up by the same name.
    native.genrule(
        name = "base_image_" + arch,
        srcs = ["Dockerfile.base"],
        outs = ["base_image_{}.marker".format(arch)],
        cmd = " && ".join([
            "docker build --platform {platform} -t {tag} -f $$(realpath $(location Dockerfile.base)) .".format(
                platform = docker_platform,
                tag = tag,
            ),
            "date > $@",
        ]),
        tags = ["manual", "no-sandbox", "requires-network", "no-remote"],
        visibility = ["//visibility:private"],
    )

def _linuxbuild(arch):
    goarch = "amd64" if arch == "x86_64" else "arm64"
    return "//tools/linuxbuild:linuxbuild_linux_" + goarch

def compile_tool(name, arch, env, patch = None):
    """Compile a tool inside Docker via the linuxbuild CLI; stdout is the tarball."""
    docker_platform, tag = _docker(arch)
    env_flags = " ".join(["-e {}={}".format(k, v) for k, v in env.items()])
    tool = _linuxbuild(arch)

    srcs = [
        ":base_image_" + arch,
        "Dockerfile.base",
        "sysroot/" + arch + "-linux-musl.tar.gz",
    ]
    patch_mount = ""
    if patch:
        srcs.append(patch)
        patch_mount = "-v $$(realpath $(location {patch})):/btop-amdgpu-sysfs.patch:ro".format(patch = patch)

    native.genrule(
        name = name + "_" + arch,
        srcs = srcs,
        tools = [tool],
        outs = ["{}_{}.tar.gz".format(name, arch)],
        cmd = """
set -euo pipefail
docker image inspect {tag} >/dev/null 2>&1 || \
  docker build --platform {platform} -t {tag} -f $$(realpath $(location Dockerfile.base)) .
toolbin=$$(mktemp)
root=$$(mktemp -d)
cp $$(realpath $(location {tool})) $$toolbin
chmod +x $$toolbin
tar -xzf $(location sysroot/{arch}-linux-musl.tar.gz) -C "$$root"
docker run --rm --pull never --platform {platform} {env} \
  -e SYSROOT=/sysroot \
  -e PKG_CONFIG_SYSROOT_DIR=/sysroot \
  -e PKG_CONFIG_LIBDIR=/sysroot/usr/lib/pkgconfig \
  -v "$$root/{arch}-linux-musl:/sysroot:ro" \
  {patch_mount} \
  -v $$toolbin:/linuxbuild:ro {tag} /linuxbuild {name} > $@
rm -f $$toolbin
rm -rf "$$root"
""".format(
            platform = docker_platform,
            env = env_flags,
            tool = tool,
            tag = tag,
            name = name,
            arch = arch,
            patch_mount = patch_mount,
        ),
        tags = ["manual", "no-sandbox", "requires-network", "no-remote"],
        visibility = ["//visibility:private"],
    )

def bundle(arch):
    """Create the final bundle assembly target."""
    native.genrule(
        name = "bundle_" + arch,
        srcs = [
            ":git_" + arch,
            ":zsh_" + arch,
            ":htop_" + arch,
            ":btop_" + arch,
            ":nvim_" + arch,
            ":make_" + arch,
            "//:versions.env",
        ],
        tools = ["//tools/assemble:assemble"],
        outs = ["devlayer-linux-{}.tar.gz".format(arch)],
        cmd = " ".join([
            "$(execpath //tools/assemble:assemble)",
            "--out $@",
            "--arch " + arch,
            "--versions $(location //:versions.env)",
            "--git $(location :git_{arch})".format(arch = arch),
            "--zsh $(location :zsh_{arch})".format(arch = arch),
            "--htop $(location :htop_{arch})".format(arch = arch),
            "--btop $(location :btop_{arch})".format(arch = arch),
            "--nvim $(location :nvim_{arch})".format(arch = arch),
            "--make $(location :make_{arch})".format(arch = arch),
        ]),
        tags = ["manual", "no-sandbox", "requires-network", "no-remote"],
        visibility = ["//visibility:public"],
    )

def _sh_quote(s):
    return "'" + s.replace("'", "'\\''") + "'"

def _scratch_smoke_impl(ctx):
    # This Bazel has no sh_test. The generated file only locates smoke.sh.
    script = ctx.actions.declare_file(ctx.label.name + ".sh")
    platform = "linux/amd64" if ctx.attr.arch == "x86_64" else "linux/arm64"
    args = " ".join([_sh_quote(a) for a in ctx.attr.cmd])
    ctx.actions.write(
        output = script,
        is_executable = True,
        content = """#!/bin/bash
set -euo pipefail
root="${{TEST_SRCDIR}}/${{TEST_WORKSPACE}}"
exec "$root/{smoke}" "$root/{archive}" {platform} {entry} {env} {expect} -- {args}
""".format(
            smoke = ctx.file._smoke.short_path,
            archive = ctx.file.archive.short_path,
            platform = platform,
            entry = _sh_quote(ctx.attr.entry),
            env = _sh_quote(ctx.attr.env),
            expect = _sh_quote(ctx.attr.expect),
            args = args,
        ),
    )
    return DefaultInfo(
        executable = script,
        runfiles = ctx.runfiles(files = [ctx.file.archive, ctx.file._smoke]),
    )

_scratch_smoke_test = rule(
    implementation = _scratch_smoke_impl,
    test = True,
    attrs = {
        "archive": attr.label(allow_single_file = True),
        "arch": attr.string(mandatory = True),
        "entry": attr.string(mandatory = True),
        "cmd": attr.string_list(mandatory = True),
        "expect": attr.string(mandatory = True),
        "env": attr.string(default = ""),
        "_smoke": attr.label(allow_single_file = True, default = "//linux:smoke/smoke.sh"),
    },
)

def _arch_of(image):
    if image.endswith("_x86_64"):
        return "x86_64"
    if image.endswith("_aarch64"):
        return "aarch64"
    fail("image must end with _x86_64 or _aarch64: " + image)

def scratch_test(name, image, entry, cmd, expect, env = ""):
    """Run image in FROM scratch. name ends with _test. cmd is the program's arguments."""
    if not name.endswith("_test"):
        fail("scratch test name must end with _test: " + name)
    _scratch_smoke_test(
        name = name,
        archive = image,
        arch = _arch_of(image),
        entry = entry,
        cmd = cmd,
        expect = expect,
        env = env,
        tags = ["manual", "no-sandbox", "no-remote"],
    )


