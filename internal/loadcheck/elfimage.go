package loadcheck

import "encoding/binary"

// IconvELF is a small ELF that records libiconv.so.2.
// Tests use it. The pack step does not.
func IconvELF() []byte {
	return elfImage([]string{"libiconv.so.2"})
}

func elfImage(names []string) []byte {
	var str []byte
	var needOff []int
	for _, name := range names {
		needOff = append(needOff, len(str))
		str = append(str, name...)
		str = append(str, 0)
	}
	if len(str) == 0 {
		str = []byte{0}
	}
	const phoff = 64
	dynOff := phoff + 2*56
	strOff := dynOff + (len(names)+2)*16
	out := make([]byte, strOff+len(str))
	out[0], out[1], out[2], out[3] = 0x7f, 'E', 'L', 'F'
	out[4], out[5], out[6] = 2, 1, 1
	binary.LittleEndian.PutUint16(out[16:18], 2)
	binary.LittleEndian.PutUint16(out[18:20], 183)
	binary.LittleEndian.PutUint32(out[20:24], 1)
	binary.LittleEndian.PutUint64(out[32:40], phoff)
	binary.LittleEndian.PutUint16(out[52:54], 64)
	binary.LittleEndian.PutUint16(out[54:56], 56)
	binary.LittleEndian.PutUint16(out[56:58], 2)
	putPhdr(out[phoff:], 1, 0, 0, uint64(len(out)))
	putPhdr(out[phoff+56:], 2, uint64(dynOff), uint64(dynOff), uint64((len(names)+2)*16))
	off := dynOff
	for _, n := range needOff {
		putDyn(out[off:], 1, uint64(n))
		off += 16
	}
	putDyn(out[off:], 5, uint64(strOff))
	off += 16
	putDyn(out[off:], 10, uint64(len(str)))
	copy(out[strOff:], str)
	return out
}

func putPhdr(b []byte, typ uint32, off, vaddr, filesz uint64) {
	binary.LittleEndian.PutUint32(b[0:4], typ)
	binary.LittleEndian.PutUint32(b[4:8], 5)
	binary.LittleEndian.PutUint64(b[8:16], off)
	binary.LittleEndian.PutUint64(b[16:24], vaddr)
	binary.LittleEndian.PutUint64(b[24:32], vaddr)
	binary.LittleEndian.PutUint64(b[32:40], filesz)
	binary.LittleEndian.PutUint64(b[40:48], filesz)
	binary.LittleEndian.PutUint64(b[48:56], 8)
}

func putDyn(b []byte, tag int64, val uint64) {
	binary.LittleEndian.PutUint64(b[0:8], uint64(tag))
	binary.LittleEndian.PutUint64(b[8:16], val)
}
