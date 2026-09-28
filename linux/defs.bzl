"""Linux bundle build macros — per-tool Docker compilation + assembly."""

load("@rules_oci//oci:defs.bzl", "oci_image", "oci_load")

def _docker(arch):
    docker_arch = "amd64" if arch == "x86_64" else "arm64"
    return "linux/" + docker_arch, "devlayer-build-base-" + docker_arch

def base_image(name, arch):
    """Create a genrule that builds the Docker base image for source compilation."""
    docker_platform, tag = _docker(arch)
    native.genrule(
        name = name,
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
    return ":" + name

def compile_tool(name, tool, arch, env, base, sysroot, linuxbuild, patch = None):
    """Compile one tool. name is the target. Returns that label."""
    docker_platform, tag = _docker(arch)
    env_flags = " ".join(["-e {}={}".format(k, v) for k, v in env.items()])
    tree = arch + "-linux-musl"

    srcs = [
        base,
        "Dockerfile.base",
        sysroot,
    ]
    patch_mount = ""
    if patch:
        srcs.append(patch)
        patch_mount = "-v $$(realpath $(location {patch})):/btop-amdgpu-sysfs.patch:ro".format(patch = patch)

    native.genrule(
        name = name,
        srcs = srcs,
        tools = [linuxbuild],
        outs = [name + ".tar.gz"],
        cmd = """
set -euo pipefail
docker image inspect {tag} >/dev/null 2>&1 || \
  docker build --platform {platform} -t {tag} -f $$(realpath $(location Dockerfile.base)) .
toolbin=$$(mktemp)
root=$$(mktemp -d)
cp $$(realpath $(location {linuxbuild})) $$toolbin
chmod +x $$toolbin
tar -xzf $(location {sysroot}) -C "$$root"
docker run --rm --pull never --platform {platform} {env} \
  -e SYSROOT=/sysroot \
  -e PKG_CONFIG_SYSROOT_DIR=/sysroot \
  -e PKG_CONFIG_LIBDIR=/sysroot/usr/lib/pkgconfig \
  -v "$$root/{tree}:/sysroot:ro" \
  {patch_mount} \
  -v $$toolbin:/linuxbuild:ro {tag} /linuxbuild {tool} > $@
rm -f $$toolbin
rm -rf "$$root"
""".format(
            platform = docker_platform,
            env = env_flags,
            linuxbuild = linuxbuild,
            sysroot = sysroot,
            tree = tree,
            tag = tag,
            tool = tool,
            patch_mount = patch_mount,
        ),
        tags = ["manual", "no-sandbox", "requires-network", "no-remote"],
        visibility = ["//visibility:private"],
    )
    return ":" + name

def bundle(name, arch, git, zsh, htop, btop, nvim, make):
    """Create the final bundle assembly target."""
    native.genrule(
        name = name,
        srcs = [
            git,
            zsh,
            htop,
            btop,
            nvim,
            make,
            "//:versions.env",
        ],
        tools = ["//tools/assemble:assemble"],
        outs = ["devlayer-linux-{}.tar.gz".format(arch)],
        cmd = " ".join([
            "$(execpath //tools/assemble:assemble)",
            "--out $@",
            "--arch " + arch,
            "--versions $(location //:versions.env)",
            "--git $(location {})".format(git),
            "--zsh $(location {})".format(zsh),
            "--htop $(location {})".format(htop),
            "--btop $(location {})".format(btop),
            "--nvim $(location {})".format(nvim),
            "--make $(location {})".format(make),
        ]),
        tags = ["manual", "no-sandbox", "requires-network", "no-remote"],
        visibility = ["//visibility:public"],
    )

def _sh_quote(s):
    return "'" + s.replace("'", "'\\''") + "'"

def image(name, arch, tars, entrypoint, env = None):
    """An OCI image. No base means scratch. tars are layers. Returns the label."""
    architecture = "amd64" if arch == "x86_64" else "arm64"
    env_arg = None
    if env:
        needs_functions = False
        lines = []
        for key, value in env.items():
            if value == "@functions":
                needs_functions = True
            else:
                lines.append("{}={}".format(key, value))
        if needs_functions:
            native.genrule(
                name = name + "_env",
                srcs = tars,
                outs = [name + ".env"],
                cmd = """
set -euo pipefail
fns=$$(tar -tzf $(location {tar}) | grep '/functions/$$' | head -n 1)
fns=$${{fns%/}}
{{
  echo FPATH=/$$fns
{extra}
}} > $@
""".format(
                    tar = tars[0],
                    extra = "\n".join(["  echo '{}'".format(line) for line in lines]),
                ),
            )
            env_arg = ":" + name + "_env"
        else:
            env_arg = env
    oci_image(
        name = name,
        os = "linux",
        architecture = architecture,
        tars = tars,
        entrypoint = entrypoint,
        env = env_arg,
    )
    return ":" + name

def _container_test_impl(ctx):
    script = ctx.actions.declare_file(ctx.label.name + ".sh")
    platform = "linux/amd64" if ctx.attr.arch == "x86_64" else "linux/arm64"
    args = " ".join([_sh_quote(a) for a in ctx.attr.cmd])
    ctx.actions.write(
        output = script,
        is_executable = True,
        content = """#!/bin/bash
set -euo pipefail
root="${{TEST_SRCDIR}}/${{TEST_WORKSPACE}}"
docker load -i "$root/{tarball}" >/dev/null
out=$(docker run --rm --network=none --platform {platform} {tag} {args})
printf '%s\\n' "$out" | grep -q {expect}
""".format(
            tarball = ctx.file.tarball.short_path,
            platform = platform,
            tag = ctx.attr.tag,
            args = args,
            expect = _sh_quote(ctx.attr.expect),
        ),
    )
    return DefaultInfo(
        executable = script,
        runfiles = ctx.runfiles(files = [ctx.file.tarball]),
    )

_container_test = rule(
    implementation = _container_test_impl,
    test = True,
    attrs = {
        "tarball": attr.label(allow_single_file = True),
        "tag": attr.string(mandatory = True),
        "arch": attr.string(mandatory = True),
        "cmd": attr.string_list(mandatory = True),
        "expect": attr.string(mandatory = True),
    },
)

def container_test(name, image, arch, cmd, expect):
    """Load an OCI image and run cmd. name ends with _test."""
    if not name.endswith("_test"):
        fail("container test name must end with _test: " + name)
    tag = "devlayer/" + name + ":test"
    oci_load(
        name = name + "_load",
        image = image,
        repo_tags = [tag],
        tags = ["manual"],
    )
    native.filegroup(
        name = name + "_tar",
        srcs = [":" + name + "_load"],
        output_group = "tarball",
        tags = ["manual"],
    )
    _container_test(
        name = name,
        tarball = ":" + name + "_tar",
        tag = tag,
        arch = arch,
        cmd = cmd,
        expect = expect,
        tags = ["manual", "no-sandbox", "no-remote"],
    )


