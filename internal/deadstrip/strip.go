// Package deadstrip drops Mach-O dylib loads that no symbol uses.
package deadstrip

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	mhMagic64 = 0xFEEDFACF

	lcReqDyld         = 0x80000000
	lcSymtab          = 0x2
	lcLoadDylib       = 0xC
	lcLoadWeakDylib   = 0x18 | lcReqDyld
	lcDyldInfo        = 0x22
	lcDyldInfoOnly    = 0x22 | lcReqDyld
	lcLoadUpwardDylib = 0x23 | lcReqDyld
	lcCodeSignature   = 0x1D
	lcReexportDylib   = 0x1F | lcReqDyld
	lcLazyLoadDylib   = 0x25
	lcChainedFixups   = 0x34 | lcReqDyld

	headerSize = 32

	bindDone           = 0x00
	bindSetOrdinalImm  = 0x10
	bindSetOrdinalUleb = 0x20
	bindSetSpecialImm  = 0x30
	bindSetSymbol      = 0x40
	bindSetType        = 0x50
	bindSetAddend      = 0x60
	bindSetSegment     = 0x70
	bindAddAddr        = 0x80
	bindDoBind         = 0x90
	bindDoBindUleb     = 0xA0
	bindDoBindImm      = 0xB0
	bindDoBindTimes    = 0xC0
	bindThreaded       = 0xD0

	maxCmds     = 4096
	maxCmdBytes = 1 << 20
	maxImports  = 1 << 20
	maxSymbols  = 1 << 22
)

// Strip drops unused Mach-O dylib loads.
// A file that is not a 64-bit Mach-O is returned as it is.
// A changed file also loses its code signature. The signature covered the old loads.
func Strip(in []byte) (out []byte, changed bool, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			out = in
			changed = false
			err = fmt.Errorf("deadstrip: panic: %v", rec)
		}
	}()
	if !isMach64(in) {
		return in, false, nil
	}
	return stripMachO(in)
}

func isMach64(in []byte) bool {
	return len(in) >= 4 && binary.LittleEndian.Uint32(in[:4]) == mhMagic64
}

type dylib struct {
	off  int
	size int
	name string
}

type fixupRef struct {
	off  int
	size int
}

type bindRef struct {
	off  int
	size int
}

type symRef struct {
	off   int
	count int
}

func stripMachO(in []byte) ([]byte, bool, error) {
	if len(in) < headerSize {
		return nil, false, errors.New("deadstrip: short mach-o header")
	}
	ncmds := binary.LittleEndian.Uint32(in[16:20])
	sizeofcmds := binary.LittleEndian.Uint32(in[20:24])
	if ncmds > maxCmds || sizeofcmds > maxCmdBytes {
		return nil, false, errors.New("deadstrip: mach-o command table is too large")
	}
	cmdEnd := headerSize + int(sizeofcmds)
	if cmdEnd > len(in) {
		return nil, false, errors.New("deadstrip: mach-o command table is truncated")
	}

	var libs []dylib
	var chains []fixupRef
	var binds []bindRef
	var syms []symRef
	var sigs []int
	off := headerSize
	for i := uint32(0); i < ncmds; i++ {
		if off+8 > cmdEnd {
			return nil, false, errors.New("deadstrip: mach-o command is truncated")
		}
		cmd := binary.LittleEndian.Uint32(in[off : off+4])
		size := int(binary.LittleEndian.Uint32(in[off+4 : off+8]))
		if size < 8 || off+size > cmdEnd {
			return nil, false, errors.New("deadstrip: mach-o command size is bad")
		}
		switch cmd {
		case lcLoadDylib, lcLoadWeakDylib, lcReexportDylib, lcLoadUpwardDylib, lcLazyLoadDylib:
			name, err := dylibName(in[off : off+size])
			if err != nil {
				return nil, false, err
			}
			libs = append(libs, dylib{off: off, size: size, name: name})
		case lcChainedFixups:
			ref, err := linkeditRef(in[off:off+size], len(in))
			if err != nil {
				return nil, false, err
			}
			chains = append(chains, fixupRef{off: ref.off, size: ref.size})
		case lcDyldInfo, lcDyldInfoOnly:
			b, err := dyldBinds(in[off:off+size], len(in))
			if err != nil {
				return nil, false, err
			}
			binds = append(binds, b...)
		case lcSymtab:
			ref, err := symtabRef(in[off:off+size], len(in))
			if err != nil {
				return nil, false, err
			}
			if ref.count > 0 {
				syms = append(syms, ref)
			}
		case lcCodeSignature:
			sigs = append(sigs, off)
		}
		off += size
	}
	if off != cmdEnd {
		return nil, false, errors.New("deadstrip: mach-o command sizes do not add up")
	}
	if len(libs) == 0 {
		return in, false, nil
	}

	used := make([]bool, len(libs)+1)
	saw := false
	for _, ch := range chains {
		if err := markChained(in, ch, used); err != nil {
			return nil, false, err
		}
		saw = true
	}
	for _, b := range binds {
		if err := markBinds(in[b.off:b.off+b.size], used); err != nil {
			return nil, false, err
		}
		saw = true
	}
	for _, s := range syms {
		if err := markSyms(in, s, used); err != nil {
			return nil, false, err
		}
		saw = true
	}
	if !saw {
		return in, false, nil
	}

	remap := make([]int, len(libs)+1)
	next := 1
	drop := 0
	for i := 1; i <= len(libs); i++ {
		if used[i] {
			remap[i] = next
			next++
			continue
		}
		drop++
	}
	if drop == 0 {
		return in, false, nil
	}

	out := append([]byte(nil), in...)
	if err := patchOrdinals(out, chains, binds, syms, remap); err != nil {
		return nil, false, err
	}
	packCommands(out, cmdEnd, sigs, libs, used)
	return out, true, nil
}

func dylibName(cmd []byte) (string, error) {
	if len(cmd) < 24 {
		return "", errors.New("deadstrip: short dylib command")
	}
	nameOff := int(binary.LittleEndian.Uint32(cmd[8:12]))
	if nameOff < 24 || nameOff >= len(cmd) {
		return "", errors.New("deadstrip: dylib name is outside the command")
	}
	end := nameOff
	for end < len(cmd) && cmd[end] != 0 {
		end++
	}
	if end == nameOff {
		return "", errors.New("deadstrip: dylib name is empty")
	}
	return string(cmd[nameOff:end]), nil
}

type span struct {
	off  int
	size int
}

func linkeditRef(cmd []byte, fileSize int) (span, error) {
	if len(cmd) < 16 {
		return span{}, errors.New("deadstrip: short linkedit command")
	}
	off := int(binary.LittleEndian.Uint32(cmd[8:12]))
	size := int(binary.LittleEndian.Uint32(cmd[12:16]))
	if off < 0 || size < 0 || off > fileSize || size > fileSize-off {
		return span{}, errors.New("deadstrip: linkedit data is outside the file")
	}
	return span{off: off, size: size}, nil
}

func dyldBinds(cmd []byte, fileSize int) ([]bindRef, error) {
	if len(cmd) < 40 {
		return nil, errors.New("deadstrip: short dyld info command")
	}
	var out []bindRef
	// bind, weak bind, lazy bind. Each pair is offset then size.
	for _, at := range []int{16, 24, 32} {
		off := int(binary.LittleEndian.Uint32(cmd[at : at+4]))
		size := int(binary.LittleEndian.Uint32(cmd[at+4 : at+8]))
		if size == 0 {
			continue
		}
		if off < 0 || off > fileSize || size > fileSize-off {
			return nil, errors.New("deadstrip: dyld bind table is outside the file")
		}
		out = append(out, bindRef{off: off, size: size})
	}
	return out, nil
}

func symtabRef(cmd []byte, fileSize int) (symRef, error) {
	if len(cmd) < 24 {
		return symRef{}, errors.New("deadstrip: short symtab command")
	}
	off := int(binary.LittleEndian.Uint32(cmd[8:12]))
	count := int(binary.LittleEndian.Uint32(cmd[12:16]))
	if count < 0 || count > maxSymbols {
		return symRef{}, errors.New("deadstrip: symtab is too large")
	}
	size := count * 16
	if count > 0 && (off < 0 || off > fileSize || size > fileSize-off) {
		return symRef{}, errors.New("deadstrip: symtab is outside the file")
	}
	return symRef{off: off, count: count}, nil
}

func markChained(in []byte, ref fixupRef, used []bool) error {
	blob := in[ref.off : ref.off+ref.size]
	if len(blob) < 28 {
		return errors.New("deadstrip: short chained fixups")
	}
	importsOff := int(binary.LittleEndian.Uint32(blob[8:12]))
	count := int(binary.LittleEndian.Uint32(blob[16:20]))
	format := int(binary.LittleEndian.Uint32(blob[20:24]))
	if count < 0 || count > maxImports {
		return errors.New("deadstrip: chained import count is too large")
	}
	width := chainedWidth(format)
	if width == 0 {
		return errors.New("deadstrip: unknown chained import format")
	}
	if importsOff < 0 || importsOff > len(blob) || count*width > len(blob)-importsOff {
		return errors.New("deadstrip: chained imports are outside the blob")
	}
	for i := 0; i < count; i++ {
		ord, err := chainedOrdinal(blob[importsOff+i*width:], format)
		if err != nil {
			return err
		}
		if err := markOrdinal(ord, used); err != nil {
			return err
		}
	}
	return nil
}

func chainedWidth(format int) int {
	switch format {
	case 1:
		return 4
	case 2:
		return 8
	case 3:
		return 16
	default:
		return 0
	}
}

// A chained ordinal uses 0x00..0xF0 as 0..240.
// 0xF1..0xFF are negative specials. The 16-bit form uses the same split at 0xFFF0.
func chainedOrdinal(b []byte, format int) (int, error) {
	switch format {
	case 1, 2:
		if len(b) < 4 {
			return 0, errors.New("deadstrip: short chained import")
		}
		raw := binary.LittleEndian.Uint32(b) & 0xff
		if raw <= 0xF0 {
			return int(raw), nil
		}
		return int(raw) - 256, nil
	case 3:
		if len(b) < 8 {
			return 0, errors.New("deadstrip: short chained import")
		}
		raw := binary.LittleEndian.Uint64(b) & 0xffff
		if raw <= 0xFFF0 {
			return int(raw), nil
		}
		return int(raw) - 65536, nil
	default:
		return 0, errors.New("deadstrip: unknown chained import format")
	}
}

func markOrdinal(ord int, used []bool) error {
	if ord <= 0 {
		return nil
	}
	if ord >= len(used) {
		return errors.New("deadstrip: dylib ordinal is outside the load list")
	}
	used[ord] = true
	return nil
}

func markBinds(buf []byte, used []bool) error {
	ord := 0
	i := 0
	for i < len(buf) {
		op := buf[i]
		i++
		imm := int(op & 0x0f)
		switch op & 0xf0 {
		case bindDone:
			ord = 0
		case bindSetOrdinalImm:
			ord = imm
		case bindSetOrdinalUleb:
			v, n, err := readULEB(buf[i:])
			if err != nil {
				return err
			}
			i += n
			ord = v
		case bindSetSpecialImm:
			if imm == 0 {
				ord = 0
			} else {
				ord = int(int8(imm | 0xF0))
			}
		case bindSetSymbol:
			for i < len(buf) && buf[i] != 0 {
				i++
			}
			if i >= len(buf) {
				return errors.New("deadstrip: bind symbol is truncated")
			}
			i++
		case bindSetType:
		case bindSetAddend:
			_, n, err := readSLEB(buf[i:])
			if err != nil {
				return err
			}
			i += n
		case bindSetSegment, bindAddAddr, bindDoBindUleb:
			_, n, err := readULEB(buf[i:])
			if err != nil {
				return err
			}
			i += n
			if op&0xf0 == bindDoBindUleb {
				if err := markOrdinal(ord, used); err != nil {
					return err
				}
			}
		case bindDoBind:
			if err := markOrdinal(ord, used); err != nil {
				return err
			}
		case bindDoBindImm:
			if err := markOrdinal(ord, used); err != nil {
				return err
			}
		case bindDoBindTimes:
			_, n, err := readULEB(buf[i:])
			if err != nil {
				return err
			}
			i += n
			_, n, err = readULEB(buf[i:])
			if err != nil {
				return err
			}
			i += n
			if err := markOrdinal(ord, used); err != nil {
				return err
			}
		case bindThreaded:
			if imm == 0 {
				_, n, err := readULEB(buf[i:])
				if err != nil {
					return err
				}
				i += n
			}
		default:
			return errors.New("deadstrip: unknown bind opcode")
		}
	}
	return nil
}

func readULEB(b []byte) (int, int, error) {
	var v, shift, n int
	for {
		if n >= len(b) || n >= 10 {
			return 0, 0, errors.New("deadstrip: bad uleb")
		}
		c := int(b[n])
		n++
		v |= (c & 0x7f) << shift
		if c&0x80 == 0 {
			return v, n, nil
		}
		shift += 7
	}
}

func readSLEB(b []byte) (int, int, error) {
	var v, shift, n int
	var c int
	for {
		if n >= len(b) || n >= 10 {
			return 0, 0, errors.New("deadstrip: bad sleb")
		}
		c = int(b[n])
		n++
		v |= (c & 0x7f) << shift
		shift += 7
		if c&0x80 == 0 {
			break
		}
	}
	if shift < 64 && c&0x40 != 0 {
		v |= -(1 << shift)
	}
	return v, n, nil
}

func markSyms(in []byte, ref symRef, used []bool) error {
	for i := 0; i < ref.count; i++ {
		at := ref.off + i*16
		nType := in[at+4]
		if nType&0x01 == 0 || nType&0x0e != 0 {
			continue
		}
		desc := binary.LittleEndian.Uint16(in[at+6 : at+8])
		ord := int(desc >> 8)
		if ord == 0 || ord >= 0xFE {
			continue
		}
		if err := markOrdinal(ord, used); err != nil {
			return err
		}
	}
	return nil
}

func patchOrdinals(out []byte, chains []fixupRef, binds []bindRef, syms []symRef, remap []int) error {
	for _, ch := range chains {
		if err := patchChained(out[ch.off:ch.off+ch.size], remap); err != nil {
			return err
		}
	}
	for _, b := range binds {
		if err := patchBinds(out[b.off:b.off+b.size], remap); err != nil {
			return err
		}
	}
	for _, s := range syms {
		if err := patchSyms(out, s, remap); err != nil {
			return err
		}
	}
	return nil
}

func patchChained(blob []byte, remap []int) error {
	if len(blob) < 28 {
		return errors.New("deadstrip: short chained fixups")
	}
	importsOff := int(binary.LittleEndian.Uint32(blob[8:12]))
	count := int(binary.LittleEndian.Uint32(blob[16:20]))
	format := int(binary.LittleEndian.Uint32(blob[20:24]))
	width := chainedWidth(format)
	for i := 0; i < count; i++ {
		at := blob[importsOff+i*width:]
		ord, err := chainedOrdinal(at, format)
		if err != nil {
			return err
		}
		if ord <= 0 {
			continue
		}
		next := remap[ord]
		if next == ord {
			continue
		}
		if err := writeChainedOrdinal(at, format, next); err != nil {
			return err
		}
	}
	return nil
}

func writeChainedOrdinal(b []byte, format, ord int) error {
	switch format {
	case 1, 2:
		if ord < 0 || ord > 0xF0 {
			return errors.New("deadstrip: chained ordinal does not fit")
		}
		v := binary.LittleEndian.Uint32(b)
		v = (v &^ 0xff) | uint32(ord)
		binary.LittleEndian.PutUint32(b, v)
	case 3:
		if ord < 0 || ord > 0xFFF0 {
			return errors.New("deadstrip: chained ordinal does not fit")
		}
		v := binary.LittleEndian.Uint64(b)
		v = (v &^ 0xffff) | uint64(ord)
		binary.LittleEndian.PutUint64(b, v)
	default:
		return errors.New("deadstrip: unknown chained import format")
	}
	return nil
}

func patchBinds(buf []byte, remap []int) error {
	i := 0
	for i < len(buf) {
		op := buf[i]
		imm := int(op & 0x0f)
		switch op & 0xf0 {
		case bindDone, bindSetType, bindDoBind, bindDoBindImm:
			i++
		case bindSetOrdinalImm:
			next, err := mapped(imm, remap)
			if err != nil {
				return err
			}
			if next != imm {
				if next > 15 {
					return errors.New("deadstrip: bind ordinal does not fit")
				}
				buf[i] = (op & 0xf0) | byte(next)
			}
			i++
		case bindSetOrdinalUleb:
			i++
			v, n, err := readULEB(buf[i:])
			if err != nil {
				return err
			}
			next, err := mapped(v, remap)
			if err != nil {
				return err
			}
			if next != v {
				if ulebLen(next) != n {
					return errors.New("deadstrip: bind ordinal width changed")
				}
				writeULEB(buf[i:i+n], next)
			}
			i += n
		case bindSetSpecialImm:
			i++
		case bindSetSymbol:
			i++
			for i < len(buf) && buf[i] != 0 {
				i++
			}
			if i >= len(buf) {
				return errors.New("deadstrip: bind symbol is truncated")
			}
			i++
		case bindSetAddend:
			i++
			_, n, err := readSLEB(buf[i:])
			if err != nil {
				return err
			}
			i += n
		case bindSetSegment, bindAddAddr, bindDoBindUleb:
			i++
			_, n, err := readULEB(buf[i:])
			if err != nil {
				return err
			}
			i += n
		case bindDoBindTimes:
			i++
			_, n, err := readULEB(buf[i:])
			if err != nil {
				return err
			}
			i += n
			_, n, err = readULEB(buf[i:])
			if err != nil {
				return err
			}
			i += n
		case bindThreaded:
			i++
			if imm == 0 {
				_, n, err := readULEB(buf[i:])
				if err != nil {
					return err
				}
				i += n
			}
		default:
			return errors.New("deadstrip: unknown bind opcode")
		}
	}
	return nil
}

func mapped(ord int, remap []int) (int, error) {
	if ord <= 0 {
		return ord, nil
	}
	if ord >= len(remap) {
		return 0, errors.New("deadstrip: dylib ordinal is outside the load list")
	}
	if remap[ord] == 0 {
		return 0, errors.New("deadstrip: bind uses a dylib that was unused")
	}
	return remap[ord], nil
}

func ulebLen(v int) int {
	n := 1
	for v >= 0x80 {
		v >>= 7
		n++
	}
	return n
}

func writeULEB(b []byte, v int) {
	for i := range b {
		c := byte(v & 0x7f)
		v >>= 7
		if i < len(b)-1 {
			c |= 0x80
		}
		b[i] = c
	}
}

func patchSyms(in []byte, ref symRef, remap []int) error {
	for i := 0; i < ref.count; i++ {
		at := ref.off + i*16
		nType := in[at+4]
		if nType&0x01 == 0 || nType&0x0e != 0 {
			continue
		}
		desc := binary.LittleEndian.Uint16(in[at+6 : at+8])
		ord := int(desc >> 8)
		if ord == 0 || ord >= 0xFE {
			continue
		}
		next, err := mapped(ord, remap)
		if err != nil {
			return err
		}
		if next == ord {
			continue
		}
		desc = (desc & 0xff) | uint16(next)<<8
		binary.LittleEndian.PutUint16(in[at+6:at+8], desc)
	}
	return nil
}

func packCommands(out []byte, cmdEnd int, sigs []int, libs []dylib, used []bool) {
	skip := map[int]bool{}
	for _, at := range sigs {
		skip[at] = true
	}
	for i, lib := range libs {
		if !used[i+1] {
			skip[lib.off] = true
		}
	}
	packed := make([]byte, 0, cmdEnd-headerSize)
	ncmds := 0
	for off := headerSize; off < cmdEnd; {
		size := int(binary.LittleEndian.Uint32(out[off+4 : off+8]))
		if !skip[off] {
			packed = append(packed, out[off:off+size]...)
			ncmds++
		}
		off += size
	}
	copy(out[headerSize:], packed)
	for i := headerSize + len(packed); i < cmdEnd; i++ {
		out[i] = 0
	}
	binary.LittleEndian.PutUint32(out[16:20], uint32(ncmds))
	binary.LittleEndian.PutUint32(out[20:24], uint32(len(packed)))
}
