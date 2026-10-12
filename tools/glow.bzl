# ---
# why: This file compiles glow from one pinned tag archive.
# uses: bazel
# how: The repo rule vendors modules. A genrule runs the host Go SDK with the network off.
# ---

"""Compile glow from the pinned source archive."""

def _host_go_key(rctx):
    os_name = rctx.os.name
    arch = rctx.os.arch
    if os_name == "mac os x":
        goos = "darwin"
    elif os_name == "linux":
        goos = "linux"
    else:
        fail("glow vendor host is " + os_name)
    if arch == "aarch64" or arch == "arm64":
        goarch = "arm64"
    elif arch == "amd64" or arch == "x86_64":
        goarch = "amd64"
    else:
        fail("glow vendor arch is " + arch)
    return goos + "_" + goarch

def _glow_src_impl(rctx):
    rctx.download_and_extract(
        url = rctx.attr.url,
        sha256 = rctx.attr.sha256,
        stripPrefix = rctx.attr.strip_prefix,
        output = "src",
    )
    key = _host_go_key(rctx)
    if key not in rctx.attr.go_urls:
        fail("no Go SDK for " + key)
    rctx.download_and_extract(
        url = rctx.attr.go_urls[key],
        sha256 = rctx.attr.go_sha256[key],
        stripPrefix = "go",
        output = "sdk",
    )
    goroot = str(rctx.path("sdk"))
    for leaf in ["home", "tmp", "gopath", "gocache"]:
        made = rctx.execute(["mkdir", "-p", str(rctx.path(leaf))])
        if made.return_code != 0:
            fail("mkdir " + leaf + "\n" + made.stderr)
    res = rctx.execute(
        [goroot + "/bin/go", "mod", "vendor"],
        environment = {
            "GOROOT": goroot,
            "GOPATH": str(rctx.path("gopath")),
            "GOCACHE": str(rctx.path("gocache")),
            "GOPROXY": "https://proxy.golang.org,direct",
            "GOSUMDB": "sum.golang.org",
            "GO111MODULE": "on",
            "GOTOOLCHAIN": "local",
            "CGO_ENABLED": "0",
            "GOFLAGS": "",
            "HOME": str(rctx.path("home")),
            "TMPDIR": str(rctx.path("tmp")),
            "PATH": "/usr/bin:/bin:/usr/sbin:/sbin",
        },
        timeout = 600,
        working_directory = str(rctx.path("src")),
    )
    if res.return_code != 0:
        fail("glow vendor failed\n" + res.stdout + res.stderr)
    rctx.file("version.txt", rctx.attr.version + "\n")
    rctx.file("BUILD.bazel", """
filegroup(
    name = "src",
    srcs = glob(["src/**"]),
    visibility = ["//visibility:public"],
)
exports_files(
    ["src/go.mod", "version.txt"],
    visibility = ["//visibility:public"],
)
""")

glow_src = repository_rule(
    implementation = _glow_src_impl,
    attrs = {
        "url": attr.string(mandatory = True),
        "sha256": attr.string(mandatory = True),
        "strip_prefix": attr.string(mandatory = True),
        "version": attr.string(mandatory = True),
        "go_urls": attr.string_dict(mandatory = True),
        "go_sha256": attr.string_dict(mandatory = True),
    },
)

def offline_go(name, goos, goarch):
    """Compile one glow binary. The action does not use the network."""
    native.genrule(
        name = name + "_compile",
        srcs = [
            "@glow_src//:src",
            "@glow_src//:src/go.mod",
            "@glow_src//:version.txt",
            "//third_party/go_sdk:go",
            "//third_party/go_sdk:go_sdk",
            "//tools:glow_build.sh",
        ],
        outs = [name + ".compile"],
        cmd = """
set -eu
out="$$PWD/$@"
script="$(execpath //tools:glow_build.sh)"
case "$$script" in /*) ;; *) script="$$PWD/$$script" ;; esac
go_bin="$(execpath //third_party/go_sdk:go)"
case "$$go_bin" in /*) ;; *) go_bin="$$PWD/$$go_bin" ;; esac
mod="$(execpath @glow_src//:src/go.mod)"
case "$$mod" in /*) ;; *) mod="$$PWD/$$mod" ;; esac
src="$$(dirname "$$mod")"
ver="$(execpath @glow_src//:version.txt)"
case "$$ver" in /*) ;; *) ver="$$PWD/$$ver" ;; esac
"$$script" "$$go_bin" "$$src" "$$out" "%s" "%s" "$$ver"
""" % (goos, goarch),
    )
    native.genrule(
        name = name,
        srcs = [":" + name + "_compile"],
        outs = [name + ".out"],
        cmd = "$(execpath //tools/deadstrip) --in $< --out $@",
        tools = ["//tools/deadstrip"],
        visibility = ["//visibility:public"],
    )
