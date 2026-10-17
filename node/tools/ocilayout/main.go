// ocilayout writes a static OCI image layout that efi-boot-shim boots.
//
//	ocilayout -out dir -layer vmlinuz=path -layer initrd.img=path \
//	    -cmdline "..." -version 7.2.11
//
// Each layer is one application/x-netboot-file blob titled by name (the Pulp
// netboot convention). The manifest annotation network.penner.boot.cmdline
// is the kernel command line. encoding/json writes every document, with
// fixed field order, so equal inputs give equal bytes.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type descriptor struct {
	MediaType    string            `json:"mediaType"`
	ArtifactType string            `json:"artifactType,omitempty"`
	Digest       string            `json:"digest"`
	Size         int64             `json:"size"`
	Data         string            `json:"data,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	Platform     *platform         `json:"platform,omitempty"`
}

type platform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
}

type manifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType"`
	ArtifactType  string            `json:"artifactType"`
	Config        descriptor        `json:"config"`
	Layers        []descriptor      `json:"layers"`
	Annotations   map[string]string `json:"annotations"`
}

type index struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Manifests     []descriptor `json:"manifests"`
}

type layers []string

func (l *layers) String() string     { return strings.Join(*l, ",") }
func (l *layers) Set(v string) error { *l = append(*l, v); return nil }

func main() {
	var ls layers
	out := flag.String("out", "", "layout directory")
	cmdline := flag.String("cmdline", "", "kernel command line")
	version := flag.String("version", "", "image version annotation")
	flag.Var(&ls, "layer", "title=path, in order")
	flag.Parse()
	if *out == "" || len(ls) == 0 {
		fail(fmt.Errorf("need -out and -layer"))
	}
	blobs := filepath.Join(*out, "blobs", "sha256")
	check(os.MkdirAll(blobs, 0o755))

	var descs []descriptor
	for _, l := range ls {
		title, path, ok := strings.Cut(l, "=")
		if !ok {
			fail(fmt.Errorf("-layer %q", l))
		}
		d, err := putFile(blobs, path)
		check(err)
		descs = append(descs, descriptor{
			MediaType:   "application/x-netboot-file",
			Digest:      d.Digest,
			Size:        d.Size,
			Annotations: map[string]string{"org.opencontainers.image.title": title},
		})
	}
	empty, err := putBytes(blobs, []byte("{}"))
	check(err)
	m := manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		ArtifactType:  "application/vnd.org.pulpproject.netboot.artifact.v1",
		Config: descriptor{
			MediaType: "application/vnd.oci.empty.v1+json",
			Digest:    empty.Digest,
			Size:      empty.Size,
			Data:      "e30=",
		},
		Layers: descs,
		Annotations: map[string]string{
			"network.penner.boot.cmdline":      *cmdline,
			"org.opencontainers.image.version": *version,
		},
	}
	mb, err := json.MarshalIndent(m, "", "  ")
	check(err)
	md, err := putBytes(blobs, append(mb, '\n'))
	check(err)
	idx := index{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.index.v1+json",
		Manifests: []descriptor{{
			MediaType:    "application/vnd.oci.image.manifest.v1+json",
			ArtifactType: m.ArtifactType,
			Digest:       md.Digest,
			Size:         md.Size,
			Platform:     &platform{Architecture: "amd64", OS: "linux"},
		}},
	}
	ib, err := json.MarshalIndent(idx, "", "  ")
	check(err)
	check(os.WriteFile(filepath.Join(*out, "index.json"), append(ib, '\n'), 0o644))
	check(os.WriteFile(filepath.Join(*out, "oci-layout"), []byte(`{"imageLayoutVersion":"1.0.0"}`+"\n"), 0o644))
}

func putBytes(dir string, b []byte) (descriptor, error) {
	sum := sha256.Sum256(b)
	hexs := hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(dir, hexs), b, 0o644); err != nil {
		return descriptor{}, err
	}
	return descriptor{Digest: "sha256:" + hexs, Size: int64(len(b))}, nil
}

func putFile(dir, path string) (descriptor, error) {
	in, err := os.Open(path)
	if err != nil {
		return descriptor{}, err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(dir, "blob")
	if err != nil {
		return descriptor{}, err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), in)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp.Name())
		return descriptor{}, err
	}
	hexs := hex.EncodeToString(h.Sum(nil))
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return descriptor{}, err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, hexs)); err != nil {
		return descriptor{}, err
	}
	return descriptor{Digest: "sha256:" + hexs, Size: n}, nil
}

func check(err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "ocilayout:", err)
	os.Exit(1)
}
