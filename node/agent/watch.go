package main

// The release watcher.
//
// A release is one OCI image manifest in the netboot index. Its version is
// the annotation org.opencontainers.image.version, a UTC timestamp that the
// publish script also puts on the kernel command line as node_release. The
// manifest digest cannot be on the command line: the command line is inside
// the manifest. So the node compares versions, and verifies the new release
// by digest before it reboots:
//
//   - the manifest bytes hash to the digest the index names, and say the same
//     version;
//   - every layer (vmlinuz, initrd.img) downloads in full, with the size and
//     sha256 the manifest names.
//
// Never on an error. A fetch error, a bad index, or a failed check is
// logged and shown in the banner; the node does not reboot.
//
// The loop guard. If the shim cannot boot the new release (a download fails
// and it boots its cached copy), the node comes back on the old release and
// sees the new one again. Without a guard it would reboot every two minutes.
// Before each reboot the agent writes {target digest, tries} to a TPM NV
// index (owner hierarchy, 64 bytes). On the next boot, if the index still
// names that target and the node is not on it, the shim failed. The agent
// then waits retry-base << (tries-1) after boot (5 min, 10, 20, ... up to
// 6 h) before it tries again. A new target resets the count. Why the TPM:
// the node keeps no disk state, every node has the TPM (sshd needs it), and
// an NV write per release is nothing for its endurance. Why not a shim flag:
// the shim does not know whether Linux came up; the node does. Without a
// usable NV index the agent does not reboot, because it could loop.

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//go:embed roots.pem
var builtinRoots []byte

const (
	annVersion = "org.opencontainers.image.version"
	annCmdline = "network.penner.boot.cmdline"
	maxIndex   = 1 << 20
	maxBlob    = 512 << 20
)

// The OCI types this needs (image-spec v1.1 index and manifest), decoded
// with encoding/json.
type descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type index struct {
	SchemaVersion int          `json:"schemaVersion"`
	Manifests     []descriptor `json:"manifests"`
}

type manifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	Config        descriptor        `json:"config"`
	Layers        []descriptor      `json:"layers"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

func watcherOff(c config) string {
	switch {
	case c.autoreboot == "off":
		return "auto-reboot off (node_autoreboot=off)"
	case c.release == "":
		return "auto-reboot off (no node_release on the kernel command line)"
	}
	return ""
}

type fetcher struct {
	client *http.Client
	base   *url.URL
	etag   string
}

func newFetcher(c config) (*fetcher, error) {
	pem := builtinRoots
	if c.roots != "" {
		b, err := os.ReadFile(c.roots)
		if err != nil {
			return nil, err
		}
		pem = b
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("no certificate in the roots")
	}
	u, err := url.Parse(c.index)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
	}
	return &fetcher{client: &http.Client{Transport: tr, Timeout: 5 * time.Minute}, base: u}, nil
}

// getIndex returns nil, nil when the index is unchanged (304).
func (f *fetcher) getIndex() (*index, error) {
	req, _ := http.NewRequest("GET", f.base.String(), nil)
	if f.etag != "" {
		req.Header.Set("If-None-Match", f.etag)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("index: HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxIndex))
	if err != nil {
		return nil, err
	}
	var idx index
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, fmt.Errorf("index: %v", err)
	}
	if idx.SchemaVersion != 2 || len(idx.Manifests) == 0 {
		return nil, errors.New("index: no manifest")
	}
	f.etag = resp.Header.Get("ETag")
	return &idx, nil
}

func parseDigest(d string) ([]byte, error) {
	hexs, ok := strings.CutPrefix(d, "sha256:")
	if !ok || len(hexs) != 64 {
		return nil, fmt.Errorf("digest %q is not sha256", d)
	}
	return hex.DecodeString(hexs)
}

// blob downloads blobs/sha256/<hex> next to the index and checks size and
// sha256 while it streams.
func (f *fetcher) blob(d descriptor, keep bool) ([]byte, error) {
	want, err := parseDigest(d.Digest)
	if err != nil {
		return nil, err
	}
	if d.Size <= 0 || d.Size > maxBlob {
		return nil, fmt.Errorf("%s: size %d", short(d.Digest), d.Size)
	}
	u := f.base.ResolveReference(&url.URL{Path: "blobs/sha256/" + hex.EncodeToString(want)})
	resp, err := f.client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", short(d.Digest), resp.StatusCode)
	}
	h := sha256.New()
	var buf bytes.Buffer
	var w io.Writer = h
	if keep {
		w = io.MultiWriter(h, &buf)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, d.Size+1))
	if err != nil {
		return nil, err
	}
	if n != d.Size {
		return nil, fmt.Errorf("%s: got %d bytes, want %d", short(d.Digest), n, d.Size)
	}
	if got := h.Sum(nil); !bytes.Equal(got, want) {
		return nil, fmt.Errorf("%s: sha256 mismatch (got %s)", short(d.Digest), hex.EncodeToString(got)[:12])
	}
	return buf.Bytes(), nil
}

// verify checks the whole release behind one index entry.
func (f *fetcher) verify(d descriptor, version string) error {
	b, err := f.blob(d, true)
	if err != nil {
		return fmt.Errorf("manifest %v", err)
	}
	var m manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("manifest: %v", err)
	}
	if v := m.Annotations[annVersion]; v != version {
		return fmt.Errorf("manifest says version %q, index says %q", v, version)
	}
	kernel := false
	for _, l := range m.Layers {
		if l.Annotations["org.opencontainers.image.title"] == "vmlinuz" {
			kernel = true
		}
		if _, err := f.blob(l, false); err != nil {
			return fmt.Errorf("layer %s: %v", l.Annotations["org.opencontainers.image.title"], err)
		}
	}
	if !kernel {
		return errors.New("manifest has no vmlinuz layer")
	}
	return nil
}

// shortErr drops Go's `Get "<url>": ` prefix; the banner names the URL.
func shortErr(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err.Error()
	}
	return err.Error()
}

func short(d string) string {
	d = strings.TrimPrefix(d, "sha256:")
	if len(d) > 12 {
		return d[:12]
	}
	return d
}

func uptime() time.Duration {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	f, _ := strconv.ParseFloat(strings.Fields(string(b))[0], 64)
	return time.Duration(f * float64(time.Second))
}

func retryWait(c config, tries int) time.Duration {
	w := c.retryBase
	for i := 1; i < tries && w < c.retryMax; i++ {
		w *= 2
	}
	if w > c.retryMax {
		w = c.retryMax
	}
	return w
}

// watch runs the check loop. It returns only with -checks, after a test
// reboot marker, or when the watcher is off: 0 normally, 1 when it could not
// start.
func watch(c config, st *status) int {
	blessOnce := func() {
		if msg, err := bless(c.efivars); err != nil {
			logk(4, "node-agent: could not bless this release for the shim: "+err.Error())
		} else if msg != "" {
			logk(5, "node-agent: "+msg)
		}
	}
	if off := watcherOff(c); off != "" {
		blessOnce()
		st.set(off, false)
		if c.checks > 0 || c.rebootMarker != "" {
			return 0
		}
		select {}
	}
	f, err := newFetcher(c)
	if err != nil {
		st.set("auto-reboot off: "+err.Error(), true)
		return 1
	}
	st.set("running "+c.release+"; checking "+c.index, false)
	failed := map[string]time.Time{}
	everOK := false
	for n := 1; ; n++ {
		wait := c.interval
		idx, err := f.getIndex()
		switch {
		case err != nil && !everOK && uptime() < c.early:
			// Right after boot DNS or the route may not answer yet (the
			// SER9's first check at 54 s timed out on 10.86.1.1:53).
			wait = c.earlyRetry
			st.set(fmt.Sprintf("waiting for the network, retrying every %s (%s)", c.earlyRetry, shortErr(err)), false)
		case err != nil:
			st.set(fmt.Sprintf("release check failed, retrying in %s; not rebooting (%s)", c.interval, shortErr(err)), true)
		case idx == nil:
			// 304: nothing new since the last look.
		default:
			if !everOK {
				blessOnce()
				everOK = true
			}
			wait = check(c, f, st, idx, failed)
		}
		if err == nil && !everOK {
			blessOnce()
			everOK = true
		}
		if c.checks > 0 && n >= c.checks {
			return 0
		}
		if wait < 0 {
			return 0 // test reboot marker written
		}
		time.Sleep(wait)
	}
}

// check handles one fresh index. It returns how long to wait, or -1 after a
// test reboot.
func check(c config, f *fetcher, st *status, idx *index, failed map[string]time.Time) time.Duration {
	d := idx.Manifests[0]
	version := d.Annotations[annVersion]
	st.mu.Lock()
	st.latest = version + " " + short(d.Digest)
	st.mu.Unlock()
	if version == "" {
		st.set("index has no release version, not rebooting", true)
		return c.interval
	}
	if version == c.release {
		st.set("up to date: "+version+" (manifest "+short(d.Digest)+")", false)
		return c.interval
	}
	if t, ok := failed[d.Digest]; ok && time.Since(t) < c.failWait {
		f.etag = "" // look again after the wait
		return c.failWait - time.Since(t)
	}
	st.set("new release "+version+" found, verifying", true)
	if err := f.verify(d, version); err != nil {
		failed[d.Digest] = time.Now()
		f.etag = ""
		st.set("new release "+version+" failed verification, not rebooting: "+err.Error(), true)
		return c.failWait
	}
	nv, err := openNV(uint32(c.nvIndex))
	if err != nil {
		f.etag = ""
		st.set("new release "+version+" verified, but no TPM NV for the loop guard, not rebooting: "+err.Error(), true)
		return c.failWait
	}
	defer nv.close()
	rec, err := nv.read()
	if err != nil {
		f.etag = ""
		st.set("new release "+version+": reading the reboot record failed, not rebooting: "+err.Error(), true)
		return c.failWait
	}
	tries := 0
	if rec.Target == d.Digest {
		tries = rec.Tries
	}
	if tries > 0 {
		need := retryWait(c, tries)
		if up := uptime(); up < need {
			f.etag = ""
			st.set(fmt.Sprintf("new release %s: rebooted for it %d time(s) but the shim booted %s; next try at uptime %s",
				version, tries, c.release, need.Round(time.Second)), true)
			return min(c.interval, need-up)
		}
	}
	when := "rebooting now"
	if c.grace > 0 {
		when = "rebooting in " + c.grace.String()
	}
	st.set(fmt.Sprintf("NEW RELEASE %s verified (manifest %s), %s", version, short(d.Digest), when), true)
	time.Sleep(c.grace)
	if err := nv.write(record{Target: d.Digest, Version: version, Tries: tries + 1}); err != nil {
		f.etag = ""
		st.set("new release "+version+": writing the reboot record failed, not rebooting: "+err.Error(), true)
		return c.failWait
	}
	if c.rebootMarker != "" {
		os.WriteFile(c.rebootMarker, []byte(d.Digest+"\n"), 0o644)
		st.set("reboot (test marker) for "+version, true)
		return -1
	}
	logk(2, "node-agent: rebooting into release "+version)
	syscall.Sync()
	if err := syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART); err != nil {
		st.set("reboot failed: "+err.Error(), true)
	}
	return c.failWait
}
