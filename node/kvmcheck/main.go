// kvmcheck runs inside the node VM. It opens /dev/kvm, asks the API
// version, and creates one VM. A pass means this kernel can host guests.
package main

import (
	"fmt"
	"os"
	"syscall"
)

// From linux/kvm.h: _IO(KVMIO, 0x00) and _IO(KVMIO, 0x01), KVMIO = 0xAE.
const (
	kvmGetAPIVersion = 0xAE00
	kvmCreateVM      = 0xAE01
	kvmAPIVersion    = 12
)

func main() {
	f, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kvmcheck:", err)
		os.Exit(1)
	}
	defer f.Close()
	v, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), kvmGetAPIVersion, 0)
	if errno != 0 || v != kvmAPIVersion {
		fmt.Fprintf(os.Stderr, "kvmcheck: KVM_GET_API_VERSION = %d (%v), want %d\n", v, errno, kvmAPIVersion)
		os.Exit(1)
	}
	vm, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), kvmCreateVM, 0)
	if errno != 0 {
		fmt.Fprintln(os.Stderr, "kvmcheck: KVM_CREATE_VM:", errno)
		os.Exit(1)
	}
	syscall.Close(int(vm))
	fmt.Printf("kvmcheck: /dev/kvm API %d, KVM_CREATE_VM ok\n", v)
}
