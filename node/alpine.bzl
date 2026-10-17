"""Alpine Linux packages as a Bazel repository.

The packages are musl builds. The test harness starts the binaries with
the musl loader from the same packages, so the host libc is not used.
"""

def _alpine_packages_impl(ctx):
    for path, sha256 in ctx.attr.packages.items():
        repo, file = path.split("/")
        # An .apk file is gzip streams of tar parts: signature, control, data.
        ctx.download_and_extract(
            url = "%s/%s/%s/%s" % (ctx.attr.mirror, repo, ctx.attr.arch, file),
            sha256 = sha256,
            type = "tar.gz",
        )
    ctx.file("BUILD.bazel", ctx.attr.build_file_content)

alpine_packages = repository_rule(
    implementation = _alpine_packages_impl,
    attrs = {
        "arch": attr.string(mandatory = True),
        "build_file_content": attr.string(mandatory = True),
        "mirror": attr.string(mandatory = True),
        "packages": attr.string_dict(mandatory = True, doc = "repo/file.apk to SHA-256"),
    },
)
