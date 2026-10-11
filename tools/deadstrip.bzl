"""One binary with unused Mach-O dylib loads removed."""

def dead_strip_binary(name, srcs):
    native.genrule(
        name = name + "_stripped",
        srcs = srcs,
        outs = [name],
        cmd = "$(execpath //tools/deadstrip) --in $< --out $@",
        tools = ["//tools/deadstrip"],
        visibility = ["//visibility:public"],
    )
