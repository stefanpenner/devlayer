"""Linux execution platforms for a local Buildbarn worker.

The worker must advertise OSFamily=Linux and the same container-image.
The runner container has no NIC.
The worker must seal each action with linux-sandbox.
That seal is private mounts, a private process list, and no NIC.
The remote protocol does not seal the action.
This repo cannot check that seal.
"""

load(":image.bzl", "rbe_images")

def declare_rbe_platforms():
    """Two Linux CPUs. Each CPU has its own Alpine digest."""
    images = rbe_images()
    cpus = [
        ("linux_arm64", "aarch64"),
        ("linux_amd64", "x86_64"),
    ]
    for name, cpu in cpus:
        native.platform(
            name = name,
            constraint_values = [
                "@platforms//os:linux",
                "@platforms//cpu:" + cpu,
            ],
            exec_properties = {
                "OSFamily": "Linux",
                "container-image": images[cpu],
            },
            visibility = ["//visibility:public"],
        )
