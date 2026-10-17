"""Layers: read-only EROFS images a node mounts by content hash.

A layer is one reproducible EROFS image. The node boots from the runtime
layer alone. Other layers mount at /layers/<name> (see node/init/init.c).
"""

# Applets the runtime toybox installs. Keep equal to toybox.miniconfig.
RUNTIME_TOYBOX = [
    "[",
    "cat",
    "chmod",
    "cp",
    "df",
    "dmesg",
    "echo",
    "env",
    "false",
    "free",
    "halt",
    "id",
    "ifconfig",
    "kill",
    "ln",
    "losetup",
    "ls",
    "md5sum",
    "mkdir",
    "mount",
    "mountpoint",
    "mv",
    "poweroff",
    "printf",
    "ps",
    "pwd",
    "reboot",
    "rm",
    "sh",
    "sha256sum",
    "sleep",
    "sync",
    "test",
    "true",
    "umount",
    "uname",
    "uptime",
]

# dropbearmulti names the runtime installs. dbclient stays in the binary for
# the SSH test but gets no name in the layer.
RUNTIME_DROPBEAR = {
    "sbin/dropbear": "../bin/dropbearmulti",
    "bin/dropbearkey": "dropbearmulti",
    "bin/scp": "dropbearmulti",
}
