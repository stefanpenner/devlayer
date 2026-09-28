"""Linux bundle build macros — per-tool Docker compilation + assembly."""

load("@versions//:versions.bzl", "VERSIONS")

def _base_image(arch):
    """Create a genrule that builds the Docker base image for source compilation."""
    docker_arch = "amd64" if arch == "x86_64" else "arm64"
    docker_platform = "linux/" + docker_arch
    tag = "devlayer-build-base-" + docker_arch

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

    return tag

def _linuxbuild(arch):
    goarch = "amd64" if arch == "x86_64" else "arm64"
    return "//tools/linuxbuild:linuxbuild_linux_" + goarch

def _docker_build(name, arch, image_tag, env = {}, patch = None):
    """Compile a tool inside Docker via the linuxbuild CLI; stdout is the tarball."""
    docker_arch = "amd64" if arch == "x86_64" else "arm64"
    docker_platform = "linux/" + docker_arch
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
            tag = image_tag,
            name = name,
            arch = arch,
            patch_mount = patch_mount,
        ),
        tags = ["manual", "no-sandbox", "requires-network", "no-remote"],
        visibility = ["//visibility:private"],
    )

def _bundle(arch):
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

def _scratch_smoke_impl(ctx):
    # This Bazel has no sh_test. The generated file only locates smoke.sh.
    script = ctx.actions.declare_file(ctx.label.name + ".sh")
    platform = "linux/amd64" if ctx.attr.arch == "x86_64" else "linux/arm64"
    ctx.actions.write(
        output = script,
        is_executable = True,
        content = """#!/bin/bash
set -euo pipefail
root="${{TEST_SRCDIR}}/${{TEST_WORKSPACE}}"
exec "$root/{smoke}" "$root/{archive}" {platform} {tool}
""".format(
            smoke = ctx.file._smoke.short_path,
            archive = ctx.file.archive.short_path,
            platform = platform,
            tool = ctx.attr.tool,
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
        "tool": attr.string(mandatory = True),
        "_smoke": attr.label(allow_single_file = True, default = "//linux:smoke/smoke.sh"),
    },
)

def _scratch_smoke(tool, arch):
    """scratch image with only this tool's files. See linux/smoke/smoke.sh."""
    _scratch_smoke_test(
        name = tool + "_smoke_" + arch,
        archive = ":{}_{}".format(tool, arch),
        arch = arch,
        tool = tool,
        tags = ["manual", "no-sandbox", "no-remote"],
    )

def linux_targets(arch):
    """Generate all Linux build targets for the given architecture."""
    image_tag = _base_image(arch)

    _docker_build("git", arch, image_tag, env = {
        "GIT_VERSION": VERSIONS["GIT"],
    })
    _docker_build("zsh", arch, image_tag, env = {
        "ZSH_VERSION": VERSIONS["ZSH"],
    })
    _docker_build("htop", arch, image_tag, env = {
        "HTOP_VERSION": VERSIONS["HTOP"],
    })
    _docker_build("btop", arch, image_tag, env = {
        "BTOP_VERSION": VERSIONS["BTOP"],
    }, patch = "patches/btop-amdgpu-sysfs.patch")
    _docker_build("nvim", arch, image_tag, env = {
        "NVIM_VERSION": VERSIONS["NVIM"],
    })
    _docker_build("make", arch, image_tag, env = {
        "MAKE_VERSION": VERSIONS["MAKE"],
    })

    for tool in ["btop", "git", "zsh", "nvim"]:
        _scratch_smoke(tool, arch)

    _bundle(arch)
