package deadstrip

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestStripDropsUnreferencedDylib(t *testing.T) {
	in := machoFixture([]string{"/usr/lib/libSystem.B.dylib", "/usr/lib/libunused.dylib"}, []int{1}, true)
	out, changed, err := Strip(in)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected a rewrite")
	}
	names := dylibNames(t, out)
	if len(names) != 1 || names[0] != "/usr/lib/libSystem.B.dylib" {
		t.Fatalf("dylibs: %q", names)
	}
	if hasCommand(out, lcCodeSignature) {
		t.Fatal("code signature still present")
	}
	if got := chainedOrdinals(t, out); len(got) != 1 || got[0] != 1 {
		t.Fatalf("ordinals: %v", got)
	}
}

func TestStripRemapsOrdinalAfterGap(t *testing.T) {
	in := machoFixture([]string{"/usr/lib/libSystem.B.dylib", "/usr/lib/libunused.dylib", "/usr/lib/libresolv.9.dylib"}, []int{1, 3}, false)
	out, changed, err := Strip(in)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected a rewrite")
	}
	names := dylibNames(t, out)
	want := []string{"/usr/lib/libSystem.B.dylib", "/usr/lib/libresolv.9.dylib"}
	if len(names) != len(want) {
		t.Fatalf("dylibs: %q", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("dylibs: %q", names)
		}
	}
	got := chainedOrdinals(t, out)
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("ordinals: %v", got)
	}
}

func TestStripKeepsReferencedDylib(t *testing.T) {
	in := machoFixture([]string{"/usr/lib/libSystem.B.dylib", "/usr/lib/libresolv.9.dylib"}, []int{1, 2}, false)
	out, changed, err := Strip(in)
	if err != nil {
		t.Fatal(err)
	}
	if changed || !bytes.Equal(out, in) {
		t.Fatal("referenced dylib was dropped")
	}
}

func TestStripLeavesNonMachO(t *testing.T) {
	in := []byte("not a binary")
	out, changed, err := Strip(in)
	if err != nil || changed || !bytes.Equal(out, in) {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
}

func TestStripNeverPanics(t *testing.T) {
	payloads := [][]byte{
		nil,
		{},
		{0xcf, 0xfa, 0xed, 0xfe},
		bytes.Repeat([]byte{0xcf, 0xfa, 0xed, 0xfe}, 8),
		{0xcf, 0xfa, 0xed, 0xfe, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0x7f},
		[]byte("\x7fELF"),
		machoFixture([]string{"/usr/lib/libSystem.B.dylib"}, nil, true),
	}
	for _, p := range payloads {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("panic on %d bytes: %v", len(p), rec)
				}
			}()
			_, _, _ = Strip(p)
		}()
	}
}

func FuzzStrip(f *testing.F) {
	f.Add(machoFixture([]string{"/usr/lib/libSystem.B.dylib", "/usr/lib/libunused.dylib"}, []int{1}, true))
	f.Add([]byte("\xcf\xfa\xed\xfe"))
	f.Fuzz(func(t *testing.T, in []byte) {
		_, _, _ = Strip(in)
	})
}

func TestTreeRewritesMachOOnly(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fd")
	note := filepath.Join(dir, "README")
	if err := os.WriteFile(bin, machoFixture([]string{"/usr/lib/libSystem.B.dylib", "/usr/lib/libunused.dylib"}, []int{1}, false), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(note, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Tree(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	names := dylibNames(t, got)
	if len(names) != 1 {
		t.Fatalf("dylibs: %q", names)
	}
	text, err := os.ReadFile(note)
	if err != nil || string(text) != "hello" {
		t.Fatalf("note changed: %q %v", text, err)
	}
}

func machoFixture(libs []string, ordinals []int, withSig bool) []byte {
	var cmds []byte
	for _, name := range libs {
		cmds = append(cmds, dylibCmd(name)...)
	}
	fix := chainedBlob(ordinals)
	// Placeholder. dataoff is filled after the command list is known.
	fixCmdAt := len(cmds)
	cmds = append(cmds, make([]byte, 16)...)
	sigCmdAt := -1
	if withSig {
		sigCmdAt = len(cmds)
		cmds = append(cmds, make([]byte, 16)...)
	}
	dataOff := headerSize + len(cmds)
	putLinkedit(cmds[fixCmdAt:], lcChainedFixups, dataOff, len(fix))
	sigOff := dataOff + len(fix)
	body := append(append(make([]byte, 0, dataOff+len(fix)+8), make([]byte, headerSize)...), cmds...)
	body = append(body, fix...)
	if withSig {
		putLinkedit(body[headerSize+sigCmdAt:], lcCodeSignature, sigOff, 4)
		body = append(body, 0, 0, 0, 0)
	}
	binary.LittleEndian.PutUint32(body[0:4], mhMagic64)
	ncmds := 1 + len(libs)
	if withSig {
		ncmds++
	}
	binary.LittleEndian.PutUint32(body[16:20], uint32(ncmds))
	binary.LittleEndian.PutUint32(body[20:24], uint32(len(cmds)))
	return body
}

func dylibCmd(name string) []byte {
	raw := append([]byte(name), 0)
	size := 24 + len(raw)
	if size%8 != 0 {
		raw = append(raw, make([]byte, 8-size%8)...)
		size = 24 + len(raw)
	}
	b := make([]byte, size)
	binary.LittleEndian.PutUint32(b[0:4], lcLoadDylib)
	binary.LittleEndian.PutUint32(b[4:8], uint32(size))
	binary.LittleEndian.PutUint32(b[8:12], 24)
	copy(b[24:], raw)
	return b
}

func putLinkedit(b []byte, cmd uint32, off, size int) {
	binary.LittleEndian.PutUint32(b[0:4], cmd)
	binary.LittleEndian.PutUint32(b[4:8], 16)
	binary.LittleEndian.PutUint32(b[8:12], uint32(off))
	binary.LittleEndian.PutUint32(b[12:16], uint32(size))
}

func chainedBlob(ordinals []int) []byte {
	n := len(ordinals)
	imports := 28
	b := make([]byte, imports+n*4)
	binary.LittleEndian.PutUint32(b[8:12], uint32(imports))
	binary.LittleEndian.PutUint32(b[12:16], uint32(imports+n*4))
	binary.LittleEndian.PutUint32(b[16:20], uint32(n))
	binary.LittleEndian.PutUint32(b[20:24], 1)
	for i, ord := range ordinals {
		binary.LittleEndian.PutUint32(b[imports+i*4:], uint32(ord))
	}
	return b
}

func dylibNames(t *testing.T, in []byte) []string {
	t.Helper()
	ncmds := binary.LittleEndian.Uint32(in[16:20])
	off := headerSize
	var names []string
	for i := uint32(0); i < ncmds; i++ {
		cmd := binary.LittleEndian.Uint32(in[off : off+4])
		size := int(binary.LittleEndian.Uint32(in[off+4 : off+8]))
		if cmd == lcLoadDylib || cmd == lcLoadWeakDylib || cmd == lcReexportDylib {
			name, err := dylibName(in[off : off+size])
			if err != nil {
				t.Fatal(err)
			}
			names = append(names, name)
		}
		off += size
	}
	return names
}

func hasCommand(in []byte, want uint32) bool {
	ncmds := binary.LittleEndian.Uint32(in[16:20])
	off := headerSize
	for i := uint32(0); i < ncmds; i++ {
		cmd := binary.LittleEndian.Uint32(in[off : off+4])
		size := int(binary.LittleEndian.Uint32(in[off+4 : off+8]))
		if cmd == want {
			return true
		}
		off += size
	}
	return false
}

func chainedOrdinals(t *testing.T, in []byte) []int {
	t.Helper()
	ncmds := binary.LittleEndian.Uint32(in[16:20])
	off := headerSize
	for i := uint32(0); i < ncmds; i++ {
		cmd := binary.LittleEndian.Uint32(in[off : off+4])
		size := int(binary.LittleEndian.Uint32(in[off+4 : off+8]))
		if cmd == lcChainedFixups {
			dataOff := int(binary.LittleEndian.Uint32(in[off+8 : off+12]))
			dataSize := int(binary.LittleEndian.Uint32(in[off+12 : off+16]))
			blob := in[dataOff : dataOff+dataSize]
			count := int(binary.LittleEndian.Uint32(blob[16:20]))
			imports := int(binary.LittleEndian.Uint32(blob[8:12]))
			var ords []int
			for n := 0; n < count; n++ {
				ords = append(ords, int(binary.LittleEndian.Uint32(blob[imports+n*4:])&0xff))
			}
			return ords
		}
		off += size
	}
	t.Fatal("no chained fixups")
	return nil
}
