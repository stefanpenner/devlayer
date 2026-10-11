// Command unpacksdk reads an Apple SDK pkg and writes a MacOSX.sdk tar.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

const (
	maxPkg     = 4 << 30
	maxTOC     = 32 << 20
	maxPayload = 4 << 30
	maxFile    = 1 << 30
	maxEntries = 500000
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("unpacksdk", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	pkg := fs.String("pkg", "", "Apple SDK pkg")
	out := fs.String("out", "", "MacOSX.sdk tar")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pkg == "" || *out == "" {
		return errors.New("unpacksdk: need --pkg and --out")
	}
	f, err := os.Open(*pkg)
	if err != nil {
		return err
	}
	defer f.Close()
	body, err := Unpack(f)
	if err != nil {
		return err
	}
	return os.WriteFile(*out, body, 0o644)
}

// Unpack reads one Apple pkg and returns a tar whose top directory is MacOSX.sdk.
func Unpack(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxPkg+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPkg {
		return nil, errors.New("unpacksdk: pkg exceeds 4GiB")
	}
	payloads, err := collectPayloads(data, "")
	if err != nil {
		return nil, err
	}
	sort.SliceStable(payloads, func(i, j int) bool {
		return strings.Contains(payloads[i].hint, "SDK") && !strings.Contains(payloads[j].hint, "SDK")
	})
	var last error
	for _, p := range payloads {
		plain, err := decodePayload(p.body)
		if err != nil {
			last = err
			continue
		}
		entries, err := parseCpio(plain)
		if err != nil {
			last = err
			continue
		}
		out, err := writeSDK(entries)
		if err != nil {
			last = err
			continue
		}
		return out, nil
	}
	if last == nil {
		last = errors.New("unpacksdk: pkg has no MacOSX.sdk")
	}
	return nil, last
}

type payload struct {
	hint string
	body []byte
}

type xarFile struct {
	Name   string    `xml:"name"`
	Type   string    `xml:"type"`
	Files  []xarFile `xml:"file"`
	Offset int64     `xml:"data>offset"`
	Length int64     `xml:"data>length"`
	Enc    xarEnc    `xml:"data>encoding"`
}

type xarEnc struct {
	Style string `xml:"style,attr"`
}

type xarDoc struct {
	Files []xarFile `xml:"toc>file"`
}

func collectPayloads(data []byte, hint string) ([]payload, error) {
	if !isXar(data) {
		return nil, errors.New("unpacksdk: not a xar pkg")
	}
	files, heap, err := parseXar(data)
	if err != nil {
		return nil, err
	}
	var out []payload
	var walk func(files []xarFile, hint string) error
	walk = func(files []xarFile, hint string) error {
		for _, f := range files {
			next := hint
			if strings.Contains(f.Name, "SDK") {
				next = f.Name
			}
			if f.Length > 0 {
				body, err := sliceHeap(heap, f)
				if err != nil {
					return err
				}
				if isXar(body) {
					nested, err := collectPayloads(body, next)
					if err != nil {
						return err
					}
					out = append(out, nested...)
				} else if isPayload(f.Name, body) {
					out = append(out, payload{hint: next, body: body})
				}
			}
			if err := walk(f.Files, next); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(files, hint); err != nil {
		return nil, err
	}
	return out, nil
}

func parseXar(data []byte) ([]xarFile, []byte, error) {
	if len(data) < 28 || string(data[:4]) != "xar!" {
		return nil, nil, errors.New("unpacksdk: bad xar header")
	}
	header := int(binary.BigEndian.Uint16(data[4:6]))
	version := binary.BigEndian.Uint16(data[6:8])
	compLen := binary.BigEndian.Uint64(data[8:16])
	rawLen := binary.BigEndian.Uint64(data[16:24])
	if version != 1 || header < 28 || header > len(data) {
		return nil, nil, errors.New("unpacksdk: bad xar header")
	}
	if compLen > maxTOC || rawLen > maxTOC || header+int(compLen) > len(data) {
		return nil, nil, errors.New("unpacksdk: bad xar toc")
	}
	zr, err := zlib.NewReader(bytes.NewReader(data[header : header+int(compLen)]))
	if err != nil {
		return nil, nil, errors.New("unpacksdk: bad xar toc")
	}
	defer zr.Close()
	toc, err := io.ReadAll(io.LimitReader(zr, maxTOC+1))
	if err != nil || len(toc) > maxTOC {
		return nil, nil, errors.New("unpacksdk: bad xar toc")
	}
	var doc xarDoc
	if err := xml.Unmarshal(toc, &doc); err != nil {
		return nil, nil, errors.New("unpacksdk: bad xar toc")
	}
	return doc.Files, data[header+int(compLen):], nil
}

func sliceHeap(heap []byte, f xarFile) ([]byte, error) {
	if f.Offset < 0 || f.Length < 0 || f.Length > maxFile {
		return nil, errors.New("unpacksdk: bad xar file")
	}
	end := f.Offset + f.Length
	if end < f.Offset || end > int64(len(heap)) {
		return nil, errors.New("unpacksdk: bad xar file")
	}
	body := heap[f.Offset:end]
	if strings.Contains(f.Enc.Style, "gzip") || (len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b) {
		return gunzip(body)
	}
	return body, nil
}

func gunzip(body []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("unpacksdk: bad gzip")
	}
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, maxPayload+1))
	if err != nil || len(out) > maxPayload {
		return nil, errors.New("unpacksdk: bad gzip")
	}
	return out, nil
}

func isXar(b []byte) bool {
	return len(b) >= 4 && string(b[:4]) == "xar!"
}

func isPayload(name string, body []byte) bool {
	if path.Base(name) == "Payload" {
		return true
	}
	return isPBZX(body) || isCpio(body)
}

func isPBZX(b []byte) bool {
	return len(b) >= 4 && string(b[:4]) == "pbzx"
}

func isCpio(b []byte) bool {
	return len(b) >= 6 && (string(b[:6]) == "070707" || string(b[:6]) == "070701")
}

func decodePayload(body []byte) ([]byte, error) {
	switch {
	case isPBZX(body):
		return decodePBZX(body)
	case isCpio(body):
		return body, nil
	default:
		return nil, errors.New("unpacksdk: unknown payload")
	}
}

func decodePBZX(b []byte) ([]byte, error) {
	if len(b) < 12 || string(b[:4]) != "pbzx" {
		return nil, errors.New("unpacksdk: bad pbzx")
	}
	flags := binary.BigEndian.Uint64(b[4:12])
	i := 12
	var out bytes.Buffer
	for flags&(1<<24) != 0 {
		if i+16 > len(b) {
			return nil, errors.New("unpacksdk: bad pbzx")
		}
		flags = binary.BigEndian.Uint64(b[i : i+8])
		length := binary.BigEndian.Uint64(b[i+8 : i+16])
		i += 16
		if length == 0 || length > maxPayload || i+int(length) > len(b) {
			return nil, errors.New("unpacksdk: bad pbzx")
		}
		chunk := b[i : i+int(length)]
		i += int(length)
		if length == 0x1000000 {
			if out.Len()+len(chunk) > maxPayload {
				return nil, errors.New("unpacksdk: payload exceeds 4GiB")
			}
			out.Write(chunk)
			continue
		}
		zr, err := xz.NewReader(bytes.NewReader(chunk))
		if err != nil {
			return nil, errors.New("unpacksdk: bad pbzx")
		}
		n, err := io.Copy(&out, io.LimitReader(zr, maxPayload+1))
		if err != nil || n > maxPayload || out.Len() > maxPayload {
			return nil, errors.New("unpacksdk: bad pbzx")
		}
	}
	return out.Bytes(), nil
}

type cpioEntry struct {
	name string
	mode int64
	body []byte
}

func parseCpio(b []byte) ([]cpioEntry, error) {
	if len(b) < 6 {
		return nil, errors.New("unpacksdk: bad cpio")
	}
	switch string(b[:6]) {
	case "070707":
		return parseODC(b)
	case "070701":
		return parseNewc(b)
	default:
		return nil, errors.New("unpacksdk: bad cpio")
	}
}

func parseODC(b []byte) ([]cpioEntry, error) {
	var out []cpioEntry
	i := 0
	for {
		if len(out) > maxEntries {
			return nil, errors.New("unpacksdk: too many cpio entries")
		}
		if i+76 > len(b) || string(b[i:i+6]) != "070707" {
			return nil, errors.New("unpacksdk: bad cpio")
		}
		mode, err := parseOct(b[i+18 : i+24])
		if err != nil {
			return nil, err
		}
		namesize, err := parseOct(b[i+59 : i+65])
		if err != nil {
			return nil, err
		}
		filesize, err := parseOct(b[i+65 : i+76])
		if err != nil {
			return nil, err
		}
		if namesize <= 0 || namesize > 1<<20 || filesize < 0 || filesize > maxFile {
			return nil, errors.New("unpacksdk: bad cpio")
		}
		start := i + 76
		nameEnd := start + int(namesize)
		dataEnd := nameEnd + int(filesize)
		if nameEnd < start || dataEnd < nameEnd || dataEnd > len(b) {
			return nil, errors.New("unpacksdk: bad cpio")
		}
		name := string(bytes.TrimRight(b[start:nameEnd], "\x00"))
		body := b[nameEnd:dataEnd]
		i = dataEnd
		if name == "TRAILER!!!" {
			return out, nil
		}
		out = append(out, cpioEntry{name: name, mode: mode, body: body})
	}
}

func parseNewc(b []byte) ([]cpioEntry, error) {
	var out []cpioEntry
	i := 0
	for {
		if len(out) > maxEntries {
			return nil, errors.New("unpacksdk: too many cpio entries")
		}
		if i+110 > len(b) || string(b[i:i+6]) != "070701" {
			return nil, errors.New("unpacksdk: bad cpio")
		}
		mode, err := parseHex(b[i+14 : i+22])
		if err != nil {
			return nil, err
		}
		filesize, err := parseHex(b[i+54 : i+62])
		if err != nil {
			return nil, err
		}
		namesize, err := parseHex(b[i+94 : i+102])
		if err != nil {
			return nil, err
		}
		if namesize <= 0 || namesize > 1<<20 || filesize < 0 || filesize > maxFile {
			return nil, errors.New("unpacksdk: bad cpio")
		}
		start := i + 110
		nameEnd := start + int(namesize)
		if nameEnd < start || nameEnd > len(b) {
			return nil, errors.New("unpacksdk: bad cpio")
		}
		name := string(bytes.TrimRight(b[start:nameEnd], "\x00"))
		data := (nameEnd + 3) &^ 3
		dataEnd := data + int(filesize)
		next := (dataEnd + 3) &^ 3
		if dataEnd < data || next < dataEnd || dataEnd > len(b) {
			return nil, errors.New("unpacksdk: bad cpio")
		}
		body := b[data:dataEnd]
		i = next
		if name == "TRAILER!!!" {
			return out, nil
		}
		out = append(out, cpioEntry{name: name, mode: mode, body: body})
	}
}

func parseOct(b []byte) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 8, 64)
	if err != nil {
		return 0, errors.New("unpacksdk: bad cpio")
	}
	return n, nil
}

func parseHex(b []byte) (int64, error) {
	n, err := strconv.ParseUint(string(b), 16, 64)
	if err != nil {
		return 0, errors.New("unpacksdk: bad cpio")
	}
	return int64(n), nil
}

func writeSDK(entries []cpioEntry) ([]byte, error) {
	by := map[string]cpioEntry{}
	var order []string
	for _, e := range entries {
		name, err := cleanName(e.name)
		if err != nil {
			return nil, err
		}
		if name == "" {
			continue
		}
		if _, ok := by[name]; ok {
			return nil, errors.New("unpacksdk: duplicate cpio path")
		}
		e.name = name
		by[name] = e
		order = append(order, name)
	}
	root, err := sdkRoot(by)
	if err != nil {
		return nil, err
	}
	var items []cpioEntry
	for _, name := range order {
		e := by[name]
		if name != root && !strings.HasPrefix(name, root+"/") {
			continue
		}
		if hasDotDot(name) {
			return nil, errors.New("unpacksdk: cpio path has ..")
		}
		rel := strings.TrimPrefix(name, root)
		rel = strings.TrimPrefix(rel, "/")
		tarName := "MacOSX.sdk"
		if rel != "" {
			tarName = path.Join("MacOSX.sdk", rel)
		}
		items = append(items, cpioEntry{name: tarName, mode: e.mode, body: e.body})
	}
	if !sdkHasInclude(items) {
		return nil, errors.New("unpacksdk: pkg has no MacOSX.sdk")
	}
	sort.Slice(items, func(i, j int) bool { return items[i].name < items[j].name })
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	wroteRoot := false
	for _, it := range items {
		if it.name == "MacOSX.sdk" {
			wroteRoot = true
		}
		if err := writeTar(tw, it); err != nil {
			return nil, err
		}
	}
	if !wroteRoot {
		if err := writeTar(tw, cpioEntry{name: "MacOSX.sdk", mode: 0o040755}); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sdkHasInclude(items []cpioEntry) bool {
	for _, it := range items {
		if it.name == "MacOSX.sdk/usr/include" || strings.HasPrefix(it.name, "MacOSX.sdk/usr/include/") {
			return true
		}
	}
	return false
}

func sdkRoot(by map[string]cpioEntry) (string, error) {
	var dir, link string
	for name, e := range by {
		if path.Base(name) != "MacOSX.sdk" {
			continue
		}
		switch e.mode & 0o170000 {
		case 0o120000:
			link = name
		case 0o040000:
			dir = name
		}
	}
	if link != "" {
		got, err := resolveLink(link, by)
		if err != nil {
			return "", err
		}
		if hasPrefix(by, got) {
			return got, nil
		}
	}
	if dir != "" && hasPrefix(by, dir) {
		return dir, nil
	}
	for name := range by {
		if name == "MacOSX.sdk" || strings.HasPrefix(name, "MacOSX.sdk/") {
			return "MacOSX.sdk", nil
		}
		const mark = "/MacOSX.sdk/"
		if i := strings.Index(name, mark); i >= 0 {
			return name[:i+len(mark)-1], nil
		}
	}
	return versionedSDKRoot(by)
}

// versionedSDKRoot finds one MacOSX<version>.sdk tree that holds usr/include.
// Command Line Tools stores the headers there. MacOSX.sdk is a separate link.
func versionedSDKRoot(by map[string]cpioEntry) (string, error) {
	seen := map[string]bool{}
	var roots []string
	for name := range by {
		root := versionedSDKDir(name)
		if root == "" || seen[root] {
			continue
		}
		seen[root] = true
		if hasPrefix(by, root) {
			roots = append(roots, root)
		}
	}
	if len(roots) == 1 {
		return roots[0], nil
	}
	if len(roots) > 1 {
		return "", errors.New("unpacksdk: pkg has more than one MacOSX.sdk")
	}
	return "", errors.New("unpacksdk: pkg has no MacOSX.sdk")
}

func versionedSDKDir(name string) string {
	parts := strings.Split(name, "/")
	for i, part := range parts {
		if part == "MacOSX.sdk" || !isSDKBase(part) {
			continue
		}
		return strings.Join(parts[:i+1], "/")
	}
	return ""
}

func isSDKBase(part string) bool {
	const prefix = "MacOSX"
	const suffix = ".sdk"
	if !strings.HasPrefix(part, prefix) || !strings.HasSuffix(part, suffix) {
		return false
	}
	mid := part[len(prefix) : len(part)-len(suffix)]
	if mid == "" {
		return true
	}
	for _, c := range mid {
		if c != '.' && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func hasPrefix(by map[string]cpioEntry, root string) bool {
	if _, ok := by[root+"/usr/include"]; ok {
		return true
	}
	prefix := root + "/usr/include/"
	for name := range by {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func resolveLink(name string, by map[string]cpioEntry) (string, error) {
	cur := name
	for hop := 0; hop < 8; hop++ {
		e, ok := by[cur]
		if !ok {
			if hasPrefix(by, cur) {
				return cur, nil
			}
			return "", errors.New("unpacksdk: broken MacOSX.sdk link")
		}
		if e.mode&0o170000 != 0o120000 {
			return cur, nil
		}
		target := string(bytes.TrimRight(e.body, "\x00"))
		next := strings.TrimPrefix(path.Clean(target), "/")
		if !strings.HasPrefix(target, "/") {
			next = path.Clean(path.Dir(cur) + "/" + target)
		}
		if next == ".." || strings.HasPrefix(next, "../") {
			return "", errors.New("unpacksdk: cpio path has ..")
		}
		cur = next
	}
	return "", errors.New("unpacksdk: broken MacOSX.sdk link")
}

func cleanName(name string) (string, error) {
	if strings.IndexByte(name, 0) >= 0 {
		return "", errors.New("unpacksdk: bad cpio name")
	}
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, "/")
	if name == "" || name == "TRAILER!!!" {
		return "", nil
	}
	return name, nil
}

func hasDotDot(name string) bool {
	for _, p := range strings.Split(name, "/") {
		if p == ".." {
			return true
		}
	}
	return false
}

func writeTar(tw *tar.Writer, e cpioEntry) error {
	hdr := &tar.Header{
		Name:    e.name,
		Mode:    e.mode & 0o777,
		ModTime: time.Unix(0, 0).UTC(),
		Format:  tar.FormatPAX,
	}
	switch e.mode & 0o170000 {
	case 0o040000:
		hdr.Typeflag = tar.TypeDir
		if !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}
	case 0o120000:
		hdr.Typeflag = tar.TypeSymlink
		hdr.Linkname = string(bytes.TrimRight(e.body, "\x00"))
	default:
		hdr.Typeflag = tar.TypeReg
		hdr.Size = int64(len(e.body))
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if hdr.Typeflag == tar.TypeReg {
		if _, err := tw.Write(e.body); err != nil {
			return err
		}
	}
	return nil
}
