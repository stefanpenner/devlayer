package linux

import "testing"

func TestHostRunsOnlyTheMatchingLinuxCPU(t *testing.T) {
	if HostCanRun("linux", "amd64", "aarch64") {
		t.Fatal("x86 Linux runs an aarch64 binary")
	}
	if !HostCanRun("linux", "amd64", "x86_64") {
		t.Fatal("x86 Linux skips an x86_64 binary")
	}
	if !HostCanRun("linux", "arm64", "aarch64") {
		t.Fatal("arm Linux skips an aarch64 binary")
	}
	if HostCanRun("linux", "arm64", "x86_64") {
		t.Fatal("arm Linux runs an x86_64 binary")
	}
	if HostCanRun("darwin", "arm64", "aarch64") {
		t.Fatal("Darwin runs a Linux binary")
	}
}
