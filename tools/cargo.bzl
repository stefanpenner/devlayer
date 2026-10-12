# ---
# why: This file builds one cargo binary without network access.
# uses: bazel
# how: A genrule copies the crate. Cargo reads the vendor directory. The link script refuses a host library.
# ---

"""One offline cargo build.

Inputs are the package tree, the vendor directory, the Rust prefix, and Zig.
An Apple target also takes the pinned SDK.
The action does not use the network.
"""

_ZIG = {
    "aarch64-apple-darwin": "aarch64-macos",
    "aarch64-unknown-linux-musl": "aarch64-linux-musl",
    "x86_64-unknown-linux-musl": "x86_64-linux-musl",
}

def offline_cargo(name, anchor, ups, vendor, bin_name, triple, release, srcs, cargo_flags = ""):
    """Build one cargo binary for a fixed target triple."""
    zig_target = _ZIG[triple]
    root = '"$(execpath %s)"' % anchor
    for _ in range(ups):
        root = "$$(dirname %s)" % root
    profile = "release" if release else "debug"
    relflag = "--release" if release else ""
    if "apple" in triple:
        # Zig joins -L with the sysroot. An absolute -L path is joined twice.
        # -liconv must be the static archive. The dylib is a host library.
        # The three headers are the libiconv 1.18 configure output for this SDK.
        # The action copies them. It compiles with zig. It does not run configure or make.
        sdk_setup = """
mkdir -p "$$work/sdk"
tar -xf "$(execpath @macos_sdk//:sdk.tar)" -C "$$work/sdk"
sdk="$$work/sdk/MacOSX.sdk"
iconv_root="$$(dirname "$(execpath @libiconv_src//:README)")"
case "$$iconv_root" in /*) ;; *) iconv_root="$$PWD/$$iconv_root" ;; esac
rm -rf "$$work/iconv-src"
mkdir -p "$$work/iconv-src"
cp -a "$$iconv_root/." "$$work/iconv-src/"
hdr="$$(dirname "$(execpath //tools:libiconv/config.h)")"
case "$$hdr" in /*) ;; *) hdr="$$PWD/$$hdr" ;; esac
cp "$$hdr/config.h" "$$work/iconv-src/lib/config.h"
cp "$$hdr/iconv.h" "$$work/iconv-src/include/iconv.h"
cp "$$hdr/localcharset.h" "$$work/iconv-src/lib/localcharset.h"
printf '%%s\\n' '#!/bin/sh' "exec '$$zig' cc -target %s --sysroot $$sdk -L/usr/lib -isystem $$sdk/usr/include \\"\\$$@\\"" > "$$work/cc-plain"
chmod 755 "$$work/cc-plain"
(
  cd "$$work/iconv-src/lib"
  "$$work/cc-plain" -I. -I../include -I.. -DBUILDING_LIBICONV -DBUILDING_LIBCHARSET -DHAVE_CONFIG_H -O2 -fvisibility=hidden -c iconv.c -o iconv.o
  "$$work/cc-plain" -I. -I../include -I.. -DBUILDING_LIBICONV -DBUILDING_LIBCHARSET -DHAVE_CONFIG_H -O2 -fvisibility=hidden -c ../libcharset/lib/localcharset.c -o localcharset.o
  "$$work/cc-plain" -I. -I../include -I.. -DBUILDING_LIBICONV -DBUILDING_LIBCHARSET -DHAVE_CONFIG_H -O2 -fvisibility=hidden -c compat.c -o compat.o
  "$$zig" ar rcs "$$work/libiconv.a" iconv.o localcharset.o compat.o
)
link="$(execpath //tools:cargo_link.sh)"
case "$$link" in /*) ;; *) link="$$PWD/$$link" ;; esac
printf '%%s\\n' '#!/bin/sh' "export ZIG='$$zig'" "export SDK='$$sdk'" "export ICONV='$$work/libiconv.a'" "export TARGET='%s'" "exec '$$link' \\"\\$$@\\"" > "$$work/cc"
chmod 755 "$$work/cc"
""" % (zig_target, zig_target)
        sdk_srcs = [
            "@macos_sdk//:sdk.tar",
            "@libiconv_src//:all",
            "@libiconv_src//:README",
            "//tools:cargo_link.sh",
            "//tools:libiconv/config.h",
            "//tools:libiconv/iconv.h",
            "//tools:libiconv/localcharset.h",
        ]
        extra_config = ""
    else:
        # Rust also ships musl crt objects. Zig must be the only crt.
        # -static omits the dynamic loader. -no-pie would cancel static PIE.
        sdk_setup = """
link="$(execpath //tools:cargo_link_linux.sh)"
case "$$link" in /*) ;; *) link="$$PWD/$$link" ;; esac
printf '%%s\\n' '#!/bin/sh' "export ZIG='$$zig'" "export TARGET='%s'" "exec '$$link' \\"\\$$@\\"" > "$$work/cc"
chmod 755 "$$work/cc"
""" % zig_target
        sdk_srcs = ["//tools:cargo_link_linux.sh"]
        extra_config = 'rustflags = ["-C", "target-feature=+crt-static", "-C", "link-self-contained=no"]\n'
    cmd = """
set -eu
out="$$PWD/$@"
# Cargo reads .cargo/config.toml from every parent.
# The work directory stays outside the home directory.
# macOS has /private/tmp. Linux has /tmp.
if [ -d /private/tmp ]; then
  tmp_root=/private/tmp
else
  tmp_root=/tmp
fi
work="$$tmp_root/devlayer-{name}-$$$$"
rm -rf "$$work"
mkdir -p "$$work/tmp" "$$work/home/cargo" "$$work/proj" "$$work/zig-cache"
export HOME="$$work/home"
export TMPDIR="$$work/tmp"
export ZIG_LOCAL_CACHE_DIR="$$work/zig-cache"
export ZIG_GLOBAL_CACHE_DIR="$$work/zig-cache"
cp -a {root}/. "$$work/proj/"
vendor="$$(dirname "$(execpath {vendor})")"
case "$$vendor" in /*) ;; *) vendor="$$PWD/$$vendor" ;; esac
rustc="$(execpath //third_party/rust:rustc)"
case "$$rustc" in /*) ;; *) rustc="$$PWD/$$rustc" ;; esac
prefix="$$(dirname "$$(dirname "$$rustc")")"
zig="$(execpath //third_party/zig:zig_bin)"
case "$$zig" in /*) ;; *) zig="$$PWD/$$zig" ;; esac
cargo="$(execpath //third_party/rust:cargo)"
case "$$cargo" in /*) ;; *) cargo="$$PWD/$$cargo" ;; esac
{sdk_setup}
printf '%s\\n' '#!/bin/sh' 'exec "$$@"' > "$$work/nowrapper"
chmod 755 "$$work/nowrapper"
cat > "$$work/home/cargo/config.toml" << EOF
[source.crates-io]
replace-with = "vendored-sources"

[source.vendored-sources]
directory = "$$vendor"

[target.{triple}]
linker = "$$work/cc"
{extra_config}

[build]
jobs = 1
rustc-wrapper = "$$work/nowrapper"
target-dir = "$$work/target"

[net]
offline = true
git-fetch-with-cli = false
EOF
export HOME="$$work/home"
export TMPDIR="$$work/tmp"
export CARGO_HOME="$$work/home/cargo"
export CARGO_TARGET_DIR="$$work/target"
export CARGO_BUILD_JOBS=1
export CARGO_BUILD_RUSTC_WRAPPER="$$work/nowrapper"
export CARGO_NET_OFFLINE=true
export CARGO_NET_GIT_FETCH_WITH_CLI=false
export PATH="$$prefix/bin:$$PATH"
export RUSTC="$$rustc"
"$$cargo" build --offline --locked --frozen --manifest-path "$$work/proj/Cargo.toml" {relflag} {cargo_flags} --target {triple}
cp "$$work/target/{triple}/{profile}/{bin_name}" "$$out"
rm -rf "$$work"
""".format(
        name = name,
        root = root,
        vendor = vendor,
        sdk_setup = sdk_setup,
        extra_config = extra_config,
        zig_target = zig_target,
        triple = triple,
        relflag = relflag,
        cargo_flags = cargo_flags,
        profile = profile,
        bin_name = bin_name,
    )
    native.genrule(
        name = name,
        srcs = srcs + sdk_srcs + [
            vendor.rsplit(":", 1)[0] + ":all",
            vendor,
        ],
        tools = [
            "//third_party/rust:rustc",
            "//third_party/rust:cargo",
            "//third_party/rust:all",
            "//third_party/zig:zig_bin",
            "//third_party/zig",
        ],
        outs = [bin_name + "-" + triple],
        cmd = cmd,
        visibility = ["//visibility:public"],
    )
