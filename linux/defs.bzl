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

def _docker_build(name, arch, image_tag, env = {}):
    """Compile a tool inside Docker via the linuxbuild CLI; stdout is the tarball."""
    docker_arch = "amd64" if arch == "x86_64" else "arm64"
    docker_platform = "linux/" + docker_arch
    env_flags = " ".join(["-e {}={}".format(k, v) for k, v in env.items()])
    tool = _linuxbuild(arch)

    native.genrule(
        name = name + "_" + arch,
        srcs = [":base_image_" + arch],
        tools = [tool],
        outs = ["{}_{}.tar.gz".format(name, arch)],
        cmd = """
set -euo pipefail
toolbin=$$(mktemp)
cp $$(realpath $(location {tool})) $$toolbin
chmod +x $$toolbin
docker run --rm --platform {platform} {env} -v $$toolbin:/linuxbuild:ro {tag} /linuxbuild {name} > $@
rm -f $$toolbin
""".format(
            platform = docker_platform,
            env = env_flags,
            tool = tool,
            tag = image_tag,
            name = name,
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

def linux_targets(arch):
    """Generate all Linux build targets for the given architecture."""
    image_tag = _base_image(arch)

    _docker_build("git", arch, image_tag, env = {
        "GIT_VERSION": VERSIONS["GIT"],
    })
    _docker_build("zsh", arch, image_tag)
    _docker_build("htop", arch, image_tag, env = {
        "HTOP_VERSION": VERSIONS["HTOP"],
    })
    _docker_build("btop", arch, image_tag, env = {
        "BTOP_VERSION": VERSIONS["BTOP"],
    })
    _docker_build("nvim", arch, image_tag)
    _docker_build("make", arch, image_tag, env = {
        "MAKE_VERSION": VERSIONS["MAKE"],
    })

    _bundle(arch)
