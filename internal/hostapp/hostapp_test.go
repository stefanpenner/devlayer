package hostapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stefanpenner/devlayer/internal/sums"
)

func TestPinsMatchTheInstaller(t *testing.T) {
	want := map[string]string{
		"darwin/arm64": "f017230c78c60a07ef4451a1eb0a92727f0b955a8fcd87aec358910c5d0c03c7",
		"darwin/amd64": "01bc6991bad122f42e4f2a5164a198d8384b944c036de112078fd51f58dc67ed",
		"linux/arm64":  "0cf567d8218995a24fb6ce4b06c07ccf58a8c517906058087355f1e2fad1969f",
		"linux/amd64":  "6277daaabab16711ab3f1961cdffad9efac5e70ac55d5076e2c86496d649d3a4",
	}
	for key, sum := range want {
		goos, goarch, _ := strings.Cut(key, "/")
		pin, err := Lookup(goos, goarch)
		if err != nil {
			t.Fatal(err)
		}
		if pin.Sum != sum {
			t.Fatalf("%s sum = %s", key, pin.Sum)
		}
		if !strings.Contains(pin.URL, Version) || !strings.HasSuffix(pin.URL, ".tar.gz") {
			t.Fatalf("%s url = %s", key, pin.URL)
		}
		if _, err := sums.Must(pin.URL); err == nil {
			t.Fatalf("%s is in the bundle pin file", pin.URL)
		}
	}
}

func TestPinRefusesWindows(t *testing.T) {
	if _, err := Lookup("windows", "amd64"); err == nil {
		t.Fatal("windows has a pin")
	}
}

func TestPlanFetchesWhenTheHostTreeIsAbsent(t *testing.T) {
	plan, err := Plan(t.TempDir(), "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Fetch || plan.Exec != "" {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestPlanExecsAStampedTree(t *testing.T) {
	data := t.TempDir()
	pin, err := Lookup("darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(data, "devlayer", "host", Name, Version)
	writeFakeApp(t, cache, Version)
	if err := WriteStamp(cache, pin.Sum); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(data, "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Fetch || plan.Exec != filepath.Join(cache, ExecRel) {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestPlanUsesTheUpstreamVersionFile(t *testing.T) {
	data := t.TempDir()
	up := filepath.Join(data, Name, "app")
	writeFakeApp(t, up, Version)
	plan, err := Plan(data, "linux", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Fetch || plan.Exec != filepath.Join(up, ExecRel) {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestPlanIgnoresAWrongVersion(t *testing.T) {
	data := t.TempDir()
	writeFakeApp(t, filepath.Join(data, Name, "app"), "v0.0.1")
	plan, err := Plan(data, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Fetch {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestPlanIgnoresABadStamp(t *testing.T) {
	data := t.TempDir()
	cache := filepath.Join(data, "devlayer", "host", Name, Version)
	writeFakeApp(t, cache, Version)
	if err := os.WriteFile(filepath.Join(cache, stampName), []byte("00\n"), 0644); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(data, "darwin", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Fetch {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestPlanIgnoresAnExecSymlinkThatLeavesTheTree(t *testing.T) {
	data := t.TempDir()
	up := filepath.Join(data, Name, "app")
	if err := os.MkdirAll(filepath.Join(up, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(up, "VERSION"), []byte(Version+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(up, ExecRel)); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(data, "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Fetch {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestWriteStampRefusesAMissingExec(t *testing.T) {
	dir := t.TempDir()
	pin, err := Lookup("linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteStamp(dir, pin.Sum); err == nil {
		t.Fatal("stamped a tree with no exec")
	}
}

func TestCleanVersion(t *testing.T) {
	good, ok := cleanVersion([]byte(Version + "\n"))
	if !ok || good != Version {
		t.Fatalf("good = %q %v", good, ok)
	}
	bad := [][]byte{
		nil,
		[]byte(""),
		[]byte(" \n"),
		[]byte(Version + " \n"),
		[]byte(Version + "\nmore\n"),
		[]byte("../" + Version),
		[]byte(Version + "\x00"),
		[]byte(strings.Repeat("v", 65)),
	}
	for _, in := range bad {
		if _, ok := cleanVersion(in); ok {
			t.Fatalf("accepted %q", in)
		}
	}
}

func TestStubDoesNotInstall(t *testing.T) {
	text := Stub()
	if !strings.HasPrefix(text, "#!/bin/sh\n") {
		t.Fatalf("stub = %s", text)
	}
	if !strings.Contains(text, `exec "$PREFIX/bin/devlayer" host terminal-browser "$@"`) {
		t.Fatalf("stub = %s", text)
	}
	for _, ban := range []string{"setup", "curl", "upgrade", "pkill"} {
		if strings.Contains(text, ban) {
			t.Fatalf("stub contains %s", ban)
		}
	}
}

func FuzzCleanVersion(f *testing.F) {
	f.Add([]byte("v0.13.4\n"))
	f.Add([]byte(""))
	f.Add([]byte("../v0.13.4"))
	f.Add([]byte("v0.13.4\x00"))
	f.Fuzz(func(t *testing.T, in []byte) {
		got, ok := cleanVersion(in)
		if !ok {
			return
		}
		if strings.ContainsAny(got, "\n\r/\\\x00") || strings.TrimSpace(got) != got || got == "" {
			t.Fatalf("accepted %q", got)
		}
	})
}

func writeFakeApp(t *testing.T, dir, version string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	body := []byte("#!/bin/sh\n")
	if err := os.WriteFile(filepath.Join(dir, ExecRel), body, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(version+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
}
