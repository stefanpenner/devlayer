package main

// What efi-boot-shim left in the UEFI variables, for dmesg and the banner:
// which shim booted this node (PennerShimBuild, set by build 3 and later),
// a self-update record (ShimUpdate), the boot trial (ShimTrial), and the
// firmware boot entries (BootCurrent, BootNext, BootOrder, Boot####).

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
)

const (
	shimVendor   = "c6f2a1d4-5b3e-4f8a-9d71-2e6b0c4a8f15"
	globalVendor = "8be4df61-93ca-11d2-aa0d-00e098032b8c"
)

// efiVar reads one variable without its 4-byte attributes. nil when absent.
func efiVar(dir, name, vendor string) []byte {
	b, err := os.ReadFile(filepath.Join(dir, name+"-"+vendor))
	if err != nil || len(b) < 4 {
		return nil
	}
	return b[4:]
}

func u16s(b []byte) []uint16 {
	out := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		out = append(out, binary.LittleEndian.Uint16(b[i:]))
	}
	return out
}

func hex4(ns []uint16) string {
	s := make([]string, len(ns))
	for i, n := range ns {
		s[i] = fmt.Sprintf("%04X", n)
	}
	return strings.Join(s, ",")
}

// bootLabel is the description of a Boot#### load option.
func bootLabel(b []byte) string {
	if len(b) < 6 {
		return "?"
	}
	var units []uint16
	for _, u := range u16s(b[6:]) {
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return string(utf16.Decode(units))
}

// shimSummary is the one banner line about the shim.
func shimSummary(dir string) string {
	if dir == "" {
		return ""
	}
	if dir == defaultEfivars {
		if _, err := os.Stat("/sys/firmware/efi"); err != nil {
			return ""
		}
		mountEfivars(dir)
	}
	s := "build 1 or 2 (no PennerShimBuild)"
	if b := efiVar(dir, "PennerShimBuild", shimVendor); b != nil {
		s = strings.TrimSpace(string(b))
	}
	if u := efiVar(dir, "ShimUpdate", shimVendor); len(u) == 41 {
		state := map[byte]string{1: "Trying", 2: "Promoting", 3: "Failed"}[u[0]]
		s += fmt.Sprintf("; ShimUpdate %s build %d %s", or(state, "?"), binary.LittleEndian.Uint64(u[33:]), hex.EncodeToString(u[1:7]))
	}
	return s
}

// efiReport is the full picture, one line each, for dmesg.
func efiReport(dir string) []string {
	sum := shimSummary(dir)
	if sum == "" {
		return nil
	}
	lines := []string{"shim: " + sum}
	if t := efiVar(dir, "ShimTrial", shimVendor); t != nil {
		lines = append(lines, "efi: ShimTrial "+hex.EncodeToString(t))
	}
	if b := efiVar(dir, "BootCurrent", globalVendor); len(b) >= 2 {
		lines = append(lines, "efi: BootCurrent "+hex4(u16s(b[:2])))
	}
	if b := efiVar(dir, "BootNext", globalVendor); len(b) >= 2 {
		lines = append(lines, "efi: BootNext "+hex4(u16s(b[:2])))
	}
	lines = append(lines, "efi: BootOrder "+hex4(u16s(efiVar(dir, "BootOrder", globalVendor))))
	names, _ := filepath.Glob(filepath.Join(dir, "Boot[0-9A-F][0-9A-F][0-9A-F][0-9A-F]-"+globalVendor))
	sort.Strings(names)
	for _, n := range names {
		v := strings.TrimSuffix(filepath.Base(n), "-"+globalVendor)
		lines = append(lines, fmt.Sprintf("efi: %s %q", v, bootLabel(efiVar(dir, v, globalVendor))))
	}
	return lines
}
