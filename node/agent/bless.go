package main

// Blessing the release for the shim.
//
// efi-boot-shim boots a new release on trial: the UEFI variable ShimTrial
// holds the manifest id and the boots left (efi-boot-shim README, "Boot
// trial"). The OS blesses the release by deleting the variable. If it is
// still there after boot_tries boots (3), the shim rolls back to the last
// blessed release and never boots this one again. A node that never blesses
// is one power cut or a few reboots away from that.
//
// node-agent blesses after the first good answer from the index: the kernel,
// init, the network, DNS and TLS all work. With the watcher off there is no
// such answer, so it blesses at start.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// The shim's vendor GUID (efi-boot-shim shim/src/trial.rs).
const trialVar = "ShimTrial-c6f2a1d4-5b3e-4f8a-9d71-2e6b0c4a8f15"

const defaultEfivars = "/sys/firmware/efi/efivars"

// bless deletes ShimTrial. It returns what it did, for the log.
func bless(dir string) (string, error) {
	if dir == "" {
		return "", nil
	}
	if dir == defaultEfivars {
		if _, err := os.Stat("/sys/firmware/efi"); err != nil {
			return "", nil // not booted by UEFI: no shim, no trial
		}
		if err := mountEfivars(dir); err != nil {
			return "", err
		}
	}
	p := filepath.Join(dir, trialVar)
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		return "", nil // blessed already, or no trial
	}
	// efivarfs marks variables immutable; clear the flag, then unlink.
	if f, err := os.Open(p); err == nil {
		var flags int
		if ioctl(f.Fd(), 0x80086601, &flags) == nil { // FS_IOC_GETFLAGS
			flags &^= 0x10                    // FS_IMMUTABLE_FL
			ioctl(f.Fd(), 0x40086602, &flags) // FS_IOC_SETFLAGS
		}
		f.Close()
	}
	if err := os.Remove(p); err != nil {
		return "", err
	}
	return "blessed this release for the shim (deleted ShimTrial)", nil
}

func ioctl(fd uintptr, req uintptr, v *int) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(unsafe.Pointer(v))); e != 0 {
		return e
	}
	return nil
}

func mountEfivars(dir string) error {
	m, _ := os.ReadFile("/proc/mounts")
	for _, l := range strings.Split(string(m), "\n") {
		if f := strings.Fields(l); len(f) > 3 && f[1] == dir {
			if f[2] == "efivarfs" && !strings.Contains(","+f[3]+",", ",ro,") {
				return nil
			}
			// Mounted read-only by hand: make it writable for the delete.
			return syscall.Mount("efivarfs", dir, "efivarfs", syscall.MS_REMOUNT|syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, "")
		}
	}
	return syscall.Mount("efivarfs", dir, "efivarfs", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, "")
}
