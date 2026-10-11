package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/ulikunitz/xz"
)

func TestUnpackGzipPayload(t *testing.T) {
	cpio := odcTree([]odcFile{
		{name: "MacOSX.sdk", mode: 0o040755},
		{name: "MacOSX.sdk/usr", mode: 0o040755},
		{name: "MacOSX.sdk/usr/include", mode: 0o040755},
		{name: "MacOSX.sdk/usr/include/stdio.h", mode: 0o100644, body: []byte("#define OK\n")},
	})
	pkg := xarBytes([]xarItem{{name: "Payload", body: gzipBytes(cpio)}})
	got, err := Unpack(bytes.NewReader(pkg))
	if err != nil {
		t.Fatal(err)
	}
	if body := tarFile(t, got, "MacOSX.sdk/usr/include/stdio.h"); string(body) != "#define OK\n" {
		t.Fatalf("%q", body)
	}
}

func TestUnpackFollowsSDKSymlink(t *testing.T) {
	cpio := odcTree([]odcFile{
		{name: "Library/Developer/CommandLineTools/SDKs", mode: 0o040755},
		{name: "Library/Developer/CommandLineTools/SDKs/MacOSX.sdk", mode: 0o120777, body: []byte("MacOSX15.sdk")},
		{name: "Library/Developer/CommandLineTools/SDKs/MacOSX15.sdk/usr/include", mode: 0o040755},
		{name: "Library/Developer/CommandLineTools/SDKs/MacOSX15.sdk/usr/include/stdio.h", mode: 0o100644, body: []byte("H\n")},
	})
	pkg := xarBytes([]xarItem{{name: "Payload", body: gzipBytes(cpio)}})
	got, err := Unpack(bytes.NewReader(pkg))
	if err != nil {
		t.Fatal(err)
	}
	if body := tarFile(t, got, "MacOSX.sdk/usr/include/stdio.h"); string(body) != "H\n" {
		t.Fatalf("%q", body)
	}
}

func TestUnpackVersionedSDK(t *testing.T) {
	cpio := odcTree([]odcFile{
		{name: "Library/Developer/CommandLineTools/SDKs/MacOSX27.sdk", mode: 0o120777, body: []byte("MacOSX27.0.sdk")},
		{name: "Library/Developer/CommandLineTools/SDKs/MacOSX27.0.sdk/usr/include", mode: 0o040755},
		{name: "Library/Developer/CommandLineTools/SDKs/MacOSX27.0.sdk/usr/include/stdio.h", mode: 0o100644, body: []byte("N\n")},
	})
	pkg := xarBytes([]xarItem{{name: "Payload", body: gzipBytes(cpio)}})
	got, err := Unpack(bytes.NewReader(pkg))
	if err != nil {
		t.Fatal(err)
	}
	if body := tarFile(t, got, "MacOSX.sdk/usr/include/stdio.h"); string(body) != "N\n" {
		t.Fatalf("%q", body)
	}
}

func TestUnpackRejectsTwoSDKs(t *testing.T) {
	cpio := odcTree([]odcFile{
		{name: "MacOSX26.sdk/usr/include/a.h", mode: 0o100644, body: []byte("a")},
		{name: "MacOSX27.0.sdk/usr/include/b.h", mode: 0o100644, body: []byte("b")},
	})
	pkg := xarBytes([]xarItem{{name: "Payload", body: gzipBytes(cpio)}})
	_, err := Unpack(bytes.NewReader(pkg))
	if err == nil || !strings.Contains(err.Error(), "more than one") {
		t.Fatal(err)
	}
}

func TestUnpackNestedPkgAndPBZX(t *testing.T) {
	cpio := odcTree([]odcFile{
		{name: "MacOSX.sdk/usr/include", mode: 0o040755},
		{name: "MacOSX.sdk/usr/include/stdio.h", mode: 0o100644, body: []byte("Z\n")},
	})
	inner := xarBytes([]xarItem{{name: "Payload", body: pbzxBytes(t, cpio)}})
	outer := xarBytes([]xarItem{{name: "CLTools_macOS_SDK.pkg", body: inner}})
	got, err := Unpack(bytes.NewReader(outer))
	if err != nil {
		t.Fatal(err)
	}
	if body := tarFile(t, got, "MacOSX.sdk/usr/include/stdio.h"); string(body) != "Z\n" {
		t.Fatalf("%q", body)
	}
}

func TestUnpackRejectsDotDot(t *testing.T) {
	cpio := odcTree([]odcFile{
		{name: "MacOSX.sdk/usr/include", mode: 0o040755},
		{name: "MacOSX.sdk/usr/include/../../evil.h", mode: 0o100644, body: []byte("no\n")},
	})
	pkg := xarBytes([]xarItem{{name: "Payload", body: gzipBytes(cpio)}})
	if _, err := Unpack(bytes.NewReader(pkg)); err == nil {
		t.Fatal("dotdot returned nil")
	}
}

func TestUnpackNeverPanics(t *testing.T) {
	for _, in := range [][]byte{
		nil,
		{},
		[]byte("xar!"),
		[]byte("xar!\x00\x1c\x00\x01"),
		[]byte("pbzx"),
		bytes.Repeat([]byte{0}, 80),
	} {
		func() {
			defer func() {
				if recover() != nil {
					t.Fatalf("panic on %q", in)
				}
			}()
			_, _ = Unpack(bytes.NewReader(in))
		}()
	}
}

func FuzzUnpack(f *testing.F) {
	cpio := odcTree([]odcFile{
		{name: "MacOSX.sdk/usr/include/stdio.h", mode: 0o100644, body: []byte("ok")},
	})
	f.Add(xarBytes([]xarItem{{name: "Payload", body: gzipBytes(cpio)}}))
	f.Add([]byte("xar!"))
	f.Fuzz(func(t *testing.T, in []byte) {
		_, _ = Unpack(bytes.NewReader(in))
	})
}

type odcFile struct {
	name string
	mode int64
	body []byte
}

func odcTree(files []odcFile) []byte {
	var b bytes.Buffer
	for _, f := range files {
		name := append([]byte(f.name), 0)
		writeOct(&b, "070707", 6)
		writeOct(&b, "0", 6)
		writeOct(&b, "0", 6)
		writeOct(&b, oct(f.mode), 6)
		writeOct(&b, "0", 6)
		writeOct(&b, "0", 6)
		writeOct(&b, "1", 6)
		writeOct(&b, "0", 6)
		writeOct(&b, "0", 11)
		writeOct(&b, oct(int64(len(name))), 6)
		writeOct(&b, oct(int64(len(f.body))), 11)
		b.Write(name)
		b.Write(f.body)
	}
	writeOct(&b, "070707", 6)
	writeOct(&b, "0", 6)
	writeOct(&b, "0", 6)
	writeOct(&b, "0", 6)
	writeOct(&b, "0", 6)
	writeOct(&b, "0", 6)
	writeOct(&b, "1", 6)
	writeOct(&b, "0", 6)
	writeOct(&b, "0", 11)
	trailer := append([]byte("TRAILER!!!"), 0)
	writeOct(&b, oct(int64(len(trailer))), 6)
	writeOct(&b, "0", 11)
	b.Write(trailer)
	return b.Bytes()
}

func oct(n int64) string {
	if n == 0 {
		return "0"
	}
	var d [32]byte
	i := len(d)
	for n > 0 {
		i--
		d[i] = byte('0' + n%8)
		n /= 8
	}
	return string(d[i:])
}

func writeOct(b *bytes.Buffer, s string, width int) {
	if len(s) > width {
		panic(s)
	}
	for i := len(s); i < width; i++ {
		b.WriteByte('0')
	}
	b.WriteString(s)
}

func gzipBytes(in []byte) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := w.Write(in); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	return b.Bytes()
}

func pbzxBytes(t *testing.T, cpio []byte) []byte {
	t.Helper()
	var xzBuf bytes.Buffer
	w, err := xz.NewWriter(&xzBuf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(cpio); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	chunk := xzBuf.Bytes()
	var b bytes.Buffer
	b.WriteString("pbzx")
	put64(&b, 1<<24)
	put64(&b, 0)
	put64(&b, uint64(len(chunk)))
	b.Write(chunk)
	return b.Bytes()
}

func put64(b *bytes.Buffer, n uint64) {
	var d [8]byte
	for i := 7; i >= 0; i-- {
		d[i] = byte(n)
		n >>= 8
	}
	b.Write(d[:])
}

type xarItem struct {
	name string
	body []byte
}

func xarBytes(items []xarItem) []byte {
	const header = 28
	var heap bytes.Buffer
	type span struct {
		name   string
		offset int
		length int
	}
	var spans []span
	for _, it := range items {
		spans = append(spans, span{name: it.name, offset: heap.Len(), length: len(it.body)})
		heap.Write(it.body)
	}
	var toc bytes.Buffer
	toc.WriteString(`<?xml version="1.0" encoding="UTF-8"?><xar><toc>`)
	for i, s := range spans {
		toc.WriteString(`<file id="`)
		toc.WriteString(oct(int64(i + 1)))
		toc.WriteString(`"><name>`)
		toc.WriteString(s.name)
		toc.WriteString(`</name><type>file</type><data><offset>`)
		toc.WriteString(strconv.Itoa(s.offset))
		toc.WriteString(`</offset><length>`)
		toc.WriteString(strconv.Itoa(s.length))
		toc.WriteString(`</length><size>`)
		toc.WriteString(strconv.Itoa(s.length))
		toc.WriteString(`</size><encoding style="application/octet-stream"/></data></file>`)
	}
	toc.WriteString(`</toc></xar>`)
	var ztoc bytes.Buffer
	zw := zlib.NewWriter(&ztoc)
	if _, err := zw.Write(toc.Bytes()); err != nil {
		panic(err)
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	var out bytes.Buffer
	out.WriteString("xar!")
	put16(&out, header)
	put16(&out, 1)
	put64(&out, uint64(ztoc.Len()))
	put64(&out, uint64(toc.Len()))
	put32(&out, 0)
	out.Write(ztoc.Bytes())
	out.Write(heap.Bytes())
	return out.Bytes()
}

func put16(b *bytes.Buffer, n uint16) {
	b.WriteByte(byte(n >> 8))
	b.WriteByte(byte(n))
}

func put32(b *bytes.Buffer, n uint32) {
	var d [4]byte
	for i := 3; i >= 0; i-- {
		d[i] = byte(n)
		n >>= 8
	}
	b.Write(d[:])
}

func tarFile(t *testing.T, archive []byte, name string) []byte {
	t.Helper()
	r := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			t.Fatalf("missing %s", name)
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name != name {
			continue
		}
		body, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
}
