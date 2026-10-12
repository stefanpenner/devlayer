package cargo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCargoWorkRootExistsOnLinux(t *testing.T) {
	body := readToolsFile(t, "cargo.bzl")
	if strings.Contains(body, `work="/private/tmp/`) {
		t.Fatal("cargo work directory is /private/tmp on every host")
	}
	if !strings.Contains(body, "tmp_root=/private/tmp") || !strings.Contains(body, "tmp_root=/tmp") {
		t.Fatalf("cargo work directory has no Linux temp root:\n%s", body)
	}
}

func TestLinuxRustStartsWithPinnedLoader(t *testing.T) {
	bzl := readToolsFile(t, "crates.bzl")
	pins := readToolsFile(t, "crate_pins.sha256")
	link := readToolsFile(t, "cargo_link_linux.sh")
	if strings.Contains(bzl, "unknown-linux-gnu") {
		t.Fatal("linux host rustc is the gnu toolchain")
	}
	if !strings.Contains(link, "-static") {
		t.Fatal("linux cargo link is not static")
	}
	for _, needle := range []string{
		"ld-musl-x86_64.so.1",
		"ld-musl-aarch64.so.1",
		"--library-path",
		"libgcc_s.so.1",
	} {
		if !strings.Contains(bzl, needle) {
			t.Errorf("crates.bzl missing %s", needle)
		}
	}
	for url, sha := range linuxLoaderPins {
		if !strings.Contains(bzl, url) {
			t.Errorf("crates.bzl missing %s", url)
		}
		if !strings.Contains(pins, sha+"  "+url) {
			t.Errorf("missing pin %s", url)
		}
	}
}

var linuxLoaderPins = map[string]string{
	"https://dl-cdn.alpinelinux.org/alpine/v3.22/main/x86_64/musl-1.2.5-r12.apk":    "4990a5e0ba312e478f94cfe431a70efef1538004eb361c8ae424516848be45bb",
	"https://dl-cdn.alpinelinux.org/alpine/v3.22/main/x86_64/libgcc-14.2.0-r6.apk":  "04f3467bc967e705221a843fe4d3de5850db826e571686e0c0ed453d38cb5c59",
	"https://dl-cdn.alpinelinux.org/alpine/v3.22/main/aarch64/musl-1.2.5-r12.apk":   "ac281d1e7f9e9c447c51e309317b975f48be6edaf3ab91ae73b959cf86703782",
	"https://dl-cdn.alpinelinux.org/alpine/v3.22/main/aarch64/libgcc-14.2.0-r6.apk": "ba1835eec3ad8a120efd3d5020e561d53553a0513763a08f509e3ce6d4baa9ca",
}

func readToolsFile(t *testing.T, name string) string {
	t.Helper()
	root := os.Getenv("TEST_SRCDIR")
	if root == "" {
		t.Fatal("TEST_SRCDIR is empty")
	}
	var hit string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || hit != "" {
			return walkErr
		}
		if d.IsDir() || d.Name() != name {
			return nil
		}
		if !strings.Contains(filepath.ToSlash(path), "/tools/"+name) {
			return nil
		}
		hit = path
		return filepath.SkipAll
	})
	if hit == "" {
		t.Fatalf("%s is missing under %s: %v", name, root, err)
	}
	raw, err := os.ReadFile(hit)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
