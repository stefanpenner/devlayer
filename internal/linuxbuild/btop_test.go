package linuxbuild

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBtopMissingVersion(t *testing.T) {
	var r recorder
	err := Btop(r.exec, r.output, nil, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "BTOP_VERSION") {
		t.Fatalf("got %v, want missing BTOP_VERSION", err)
	}
}

func TestFindNamed(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "build", "src")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(nested, "btop")
	if err := os.WriteFile(want, []byte("bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "other"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := FindNamed(filepath.Join(root, "build"), "btop")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("FindNamed = %q, want %q", got, want)
	}

	_, err = FindNamed(root, "missing")
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestBtop(t *testing.T) {
	chdirTemp(t)
	ver := "1.4.0"
	dir := "btop-" + ver
	binDir := filepath.Join(dir, "build")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "btop"), []byte("fake-btop"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove("/tmp/btop") })

	var r recorder
	if err := Btop(r.exec, r.output, map[string]string{"BTOP_VERSION": ver}, io.Discard); err != nil {
		t.Fatal(err)
	}

	url := "https://github.com/aristocratos/btop/archive/refs/tags/v" + ver + ".tar.gz"
	r.has(t, url)
	r.hasSeq(t,
		"curl",
		"tar",
		dir+": cmake -B build -DCMAKE_BUILD_TYPE=Release -DBTOP_STATIC=ON -DBTOP_GPU=OFF -DBTOP_LTO=ON",
		dir+": cmake --build build "+jobsArg(),
		"strip /tmp/btop",
		"tar czf - -C /tmp btop",
	)

	got, err := os.ReadFile("/tmp/btop")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "fake-btop" {
		t.Fatalf("/tmp/btop = %q", got)
	}
}
