"""Pinned Rust toolchains and vendored crates.

The Go compiler matches the host. Rust archives stay pinned per triple.
Build actions use the outputs.
They do not download and they do not call host cargo.
"""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

_VER = "1.90.0"
_DATE = "2025-09-18"
_GO_VER = "1.27.1"
_TRIPLES = [
    "aarch64-apple-darwin",
    "aarch64-unknown-linux-musl",
    "x86_64-unknown-linux-musl",
]

def _parse_sums(text):
    sums = {}
    for raw in text.split("\n"):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        sha, url = line.split("  ", 1)
        sums[url.strip()] = sha.strip()
    return sums

def _pin(sums, url):
    got = sums.get(url)
    if not got:
        fail("no sha256 pin for " + url)
    return got

def _host_go_url(os_name, arch):
    """Return the Go SDK archive for the machine that runs this repo rule."""
    if os_name == "mac os x":
        goos = "darwin"
    elif os_name == "linux":
        goos = "linux"
    else:
        fail("cratevendor host is " + os_name)
    if arch == "aarch64" or arch == "arm64":
        goarch = "arm64"
    elif arch == "amd64" or arch == "x86_64":
        goarch = "amd64"
    else:
        fail("cratevendor arch is " + arch)
    return "https://go.dev/dl/go%s.%s-%s.tar.gz" % (_GO_VER, goos, goarch)

def _rust_url(triple):
    return "https://static.rust-lang.org/dist/%s/rust-%s-%s.tar.xz" % (_DATE, _VER, triple)

def _loader_spec(triple):
    """Return the musl loader pins for a Linux host toolchain.

    The musl rustc records /lib/ld-musl-*.so.1.
    That file is not in the rust archive.
    Ubuntu does not have it.
    These Alpine packages supply the loader and libgcc.
    The host libc stays unused.
    """
    if triple == "x86_64-unknown-linux-musl":
        return {
            "ld": "ld-musl-x86_64.so.1",
            "musl": "https://dl-cdn.alpinelinux.org/alpine/v3.22/main/x86_64/musl-1.2.5-r12.apk",
            "libgcc": "https://dl-cdn.alpinelinux.org/alpine/v3.22/main/x86_64/libgcc-14.2.0-r6.apk",
        }
    if triple == "aarch64-unknown-linux-musl":
        return {
            "ld": "ld-musl-aarch64.so.1",
            "musl": "https://dl-cdn.alpinelinux.org/alpine/v3.22/main/aarch64/musl-1.2.5-r12.apk",
            "libgcc": "https://dl-cdn.alpinelinux.org/alpine/v3.22/main/aarch64/libgcc-14.2.0-r6.apk",
        }
    return None

def _std_url(triple):
    return "https://static.rust-lang.org/dist/%s/rust-std-%s-%s.tar.xz" % (_DATE, _VER, triple)

def _rust_impl(rctx):
    rctx.download_and_extract(
        url = rctx.attr.url,
        sha256 = rctx.attr.sha256,
        stripPrefix = rctx.attr.strip_prefix,
    )
    rctx.file("home/.keep", "")
    rctx.file("tmp/.keep", "")
    prefix = str(rctx.path("prefix"))
    env = {
        "HOME": str(rctx.path("home")),
        "TMPDIR": str(rctx.path("tmp")),
    }
    # This installer has --prefix and --disable-ldconfig.
    # It has no --without-modify-path flag.
    host_std = "rust-std-" + rctx.attr.host_triple
    res = rctx.execute(
        [
            "./install.sh",
            "--prefix=" + prefix,
            "--disable-ldconfig",
            "--components=rustc,cargo,llvm-tools-preview," + host_std,
        ],
        environment = env,
        timeout = 600,
    )
    if res.return_code:
        fail("rust install failed: " + res.stderr + res.stdout)
    for spec in rctx.attr.extra_std:
        url, sha, strip = spec.split(" ")
        dest = "std-" + strip
        rctx.download_and_extract(url = url, sha256 = sha, stripPrefix = strip, output = dest)
        res = rctx.execute(
            ["./install.sh", "--prefix=" + prefix, "--disable-ldconfig"],
            working_directory = dest,
            environment = env,
            timeout = 600,
        )
        if res.return_code:
            fail("rust-std install failed: " + res.stderr + res.stdout)
    _install_musl_loader(rctx)
    rctx.file("BUILD.bazel", """
filegroup(
    name = "all",
    srcs = glob(["prefix/**"]),
    visibility = ["//visibility:public"],
)
exports_files(
    ["prefix/bin/rustc", "prefix/bin/cargo"],
    visibility = ["//visibility:public"],
)
""")

def _run(rctx, argv, label):
    res = rctx.execute(argv)
    if res.return_code:
        fail(label + ": " + res.stderr + res.stdout)

def _install_musl_loader(rctx):
    if not rctx.attr.musl_loader_url:
        return
    rctx.download(
        url = rctx.attr.musl_loader_url,
        output = "musl.apk",
        sha256 = rctx.attr.musl_loader_sha256,
    )
    rctx.download(
        url = rctx.attr.libgcc_url,
        output = "libgcc.apk",
        sha256 = rctx.attr.libgcc_sha256,
    )
    _run(rctx, ["mkdir", "-p", "prefix/musl", "apk-musl", "apk-gcc"], "musl loader mkdir")
    _run(rctx, ["tar", "-xzf", "musl.apk", "-C", "apk-musl"], "musl apk extract")
    _run(rctx, ["tar", "-xzf", "libgcc.apk", "-C", "apk-gcc"], "libgcc apk extract")
    ld = rctx.attr.loader_name
    _run(rctx, ["cp", "-L", "apk-musl/lib/" + ld, "prefix/musl/" + ld], "musl loader copy")
    _run(rctx, ["cp", "-L", "apk-gcc/usr/lib/libgcc_s.so.1", "prefix/musl/libgcc_s.so.1"], "libgcc copy")
    _run(rctx, ["chmod", "755", "prefix/musl/" + ld], "musl loader chmod")
    # libc.so is this loader. --library-path finds libgcc. RPATH finds rustc.
    script = """#!/bin/sh
here=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
exec "$here/../musl/%s" --library-path "$here/../musl" "$here/%s.bin" "$@"
"""
    for name in ["cargo", "rustc"]:
        _run(rctx, ["cp", "-L", "prefix/bin/" + name, "prefix/bin/" + name + ".bin"], name + " copy")
        _run(rctx, ["chmod", "755", "prefix/bin/" + name + ".bin"], name + " chmod")
        rctx.file("prefix/bin/" + name, script % (ld, name), executable = True)
    rctx.delete("apk-musl")
    rctx.delete("apk-gcc")
    rctx.delete("musl.apk")
    rctx.delete("libgcc.apk")

rust_toolchain = repository_rule(
    implementation = _rust_impl,
    attrs = {
        "extra_std": attr.string_list(),
        "host_triple": attr.string(mandatory = True),
        "libgcc_sha256": attr.string(),
        "libgcc_url": attr.string(),
        "loader_name": attr.string(),
        "musl_loader_sha256": attr.string(),
        "musl_loader_url": attr.string(),
        "sha256": attr.string(mandatory = True),
        "strip_prefix": attr.string(mandatory = True),
        "url": attr.string(mandatory = True),
    },
)

def _vendor_impl(rctx):
    go = rctx.path(rctx.attr._go)
    lock = rctx.path(rctx.attr.lock)
    # A lock edit must fetch again. The label alone did not restart this rule.
    rctx.watch(lock)
    goroot = str(go.dirname.dirname)
    rctx.file("gocache/.keep", "")
    rctx.file("gopath/src/cratevendor/main.go", rctx.read(rctx.path(rctx.attr._src)))
    res = rctx.execute(
        [str(go), "build", "-o", str(rctx.path("cratevendor")), "cratevendor"],
        environment = {
            "CGO_ENABLED": "0",
            "GO111MODULE": "off",
            "GOCACHE": str(rctx.path("gocache")),
            "GOPATH": str(rctx.path("gopath")),
            "GOROOT": goroot,
        },
        timeout = 180,
    )
    if res.return_code:
        fail("cratevendor build failed: " + res.stderr + res.stdout)
    res = rctx.execute(
        [str(rctx.path("cratevendor")), "--lock", str(lock), "--out", "vendor"],
        timeout = 900,
    )
    if res.return_code:
        fail("cratevendor failed: " + res.stderr + res.stdout)
    rctx.file("BUILD.bazel", """
filegroup(
    name = "all",
    srcs = glob(["vendor/**"]),
    visibility = ["//visibility:public"],
)
exports_files(["vendor/ANCHOR"], visibility = ["//visibility:public"])
""")

crate_vendor = repository_rule(
    implementation = _vendor_impl,
    attrs = {
        "lock": attr.label(mandatory = True),
        "_go": attr.label(default = "@cratevendor_go//:bin/go"),
        "_src": attr.label(default = "//tools/cratevendor:main.go"),
    },
)

def _crate_repos_impl(module_ctx):
    sums = _parse_sums(module_ctx.read(Label("//tools:crate_pins.sha256")))
    go_url = _host_go_url(module_ctx.os.name, module_ctx.os.arch)
    http_archive(
        name = "cratevendor_go",
        urls = [go_url],
        sha256 = _pin(sums, go_url),
        strip_prefix = "go",
        build_file_content = """
exports_files(["bin/go"], visibility = ["//visibility:public"])
""",
    )
    for triple in _TRIPLES:
        extra = []
        for std_triple in _TRIPLES:
            if std_triple == triple:
                continue
            url = _std_url(std_triple)
            extra.append("%s %s %s" % (url, _pin(sums, url), "rust-std-%s-%s" % (_VER, std_triple)))
        url = _rust_url(triple)
        name = "rust_" + triple.replace("-", "_")
        loader = _loader_spec(triple)
        musl_url = loader["musl"] if loader else ""
        libgcc_url = loader["libgcc"] if loader else ""
        rust_toolchain(
            name = name,
            url = url,
            sha256 = _pin(sums, url),
            strip_prefix = "rust-%s-%s" % (_VER, triple),
            host_triple = triple,
            extra_std = extra,
            musl_loader_url = musl_url,
            musl_loader_sha256 = _pin(sums, musl_url) if musl_url else "",
            libgcc_url = libgcc_url,
            libgcc_sha256 = _pin(sums, libgcc_url) if libgcc_url else "",
            loader_name = loader["ld"] if loader else "",
        )
    eza_url = "https://github.com/eza-community/eza/archive/refs/tags/v0.23.5.tar.gz"
    http_archive(
        name = "eza_src",
        urls = [eza_url],
        sha256 = _pin(sums, eza_url),
        strip_prefix = "eza-0.23.5",
        build_file_content = """
exports_files(["Cargo.toml", "Cargo.lock"], visibility = ["//visibility:public"])
filegroup(
    name = "all",
    srcs = glob(["**"]),
    visibility = ["//visibility:public"],
)
""",
    )
    crate_vendor(
        name = "bininfo_crates",
        lock = "//bininfo:Cargo.lock",
    )
    crate_vendor(
        name = "hoststub_crates",
        lock = "//hoststub:Cargo.lock",
    )
    crate_vendor(
        name = "eza_crates",
        lock = "@eza_src//:Cargo.lock",
    )

crate_repos = module_extension(implementation = _crate_repos_impl)
