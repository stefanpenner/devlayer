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

# OpenSSH programs in the runtime layer, by path. ssh and ssh-keygen are
# built too, for the tests; they are not in the layer.
RUNTIME_OPENSSH = {
    "sbin/sshd": "sshd",
    "libexec/sshd-session": "sshd-session",
    "libexec/sshd-auth": "sshd-auth",
}
