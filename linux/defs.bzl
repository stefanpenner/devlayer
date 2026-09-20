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

def _docker_build(name, arch, image_tag, script_file, env = {}):
    """Create a genrule that compiles a tool inside Docker and outputs a tarball.

    The build script is piped via stdin to docker run. Versions are passed
    as environment variables to avoid any file mounting.
    """
    docker_arch = "amd64" if arch == "x86_64" else "arm64"
    docker_platform = "linux/" + docker_arch

    env_flags = " ".join(["-e {}={}".format(k, v) for k, v in env.items()])

    native.genrule(
        name = name + "_" + arch,
        srcs = [":base_image_" + arch, script_file],
        outs = ["{}_{}.tar.gz".format(name, arch)],
        cmd = "docker run --rm -i --platform {platform} {env} {tag} /bin/sh < $$(realpath $(location {script})) > $@".format(
            platform = docker_platform,
            env = env_flags,
            tag = image_tag,
            script = script_file,
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
            "assemble.sh",
            "//:versions.env",
            "//scripts:download-binaries.sh",
        ],
        outs = ["devlayer-linux-{}.tar.gz".format(arch)],
        cmd = " ".join([
            "bash $$(realpath $(location assemble.sh))",
            "$@",
            arch,
            "$$(realpath $(location //:versions.env))",
            "$$(realpath $(location //scripts:download-binaries.sh))",
            "$$(realpath $(location :git_{arch}))".format(arch = arch),
            "$$(realpath $(location :zsh_{arch}))".format(arch = arch),
            "$$(realpath $(location :htop_{arch}))".format(arch = arch),
            "$$(realpath $(location :btop_{arch}))".format(arch = arch),
            "$$(realpath $(location :nvim_{arch}))".format(arch = arch),
            "$$(realpath $(location :make_{arch}))".format(arch = arch),
        ]),
        tags = ["manual", "no-sandbox", "requires-network", "no-remote"],
        visibility = ["//visibility:public"],
    )

def linux_targets(arch):
    """Generate all Linux build targets for the given architecture."""
    image_tag = _base_image(arch)

    _docker_build("git", arch, image_tag, "scripts/build_git.sh", env = {
        "GIT_VERSION": VERSIONS["GIT"],
    })
    _docker_build("zsh", arch, image_tag, "scripts/build_zsh.sh", env = {
        "ZSH_VERSION": VERSIONS["ZSH"],
    })
    _docker_build("htop", arch, image_tag, "scripts/build_htop.sh", env = {
        "HTOP_VERSION": VERSIONS["HTOP"],
    })
    _docker_build("btop", arch, image_tag, "scripts/build_btop.sh", env = {
        "BTOP_VERSION": VERSIONS["BTOP"],
    })
    _docker_build("nvim", arch, image_tag, "scripts/build_nvim.sh", env = {
        "NVIM_VERSION": VERSIONS["NVIM"],
    })
    _docker_build("make", arch, image_tag, "scripts/build_make.sh", env = {
        "MAKE_VERSION": VERSIONS["MAKE"],
    })

    _bundle(arch)
