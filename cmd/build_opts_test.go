package cmd

import "testing"

func TestParseBuildOptsDefaultOS(t *testing.T) {
	opts, err := parseBuildOpts(nil, "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if opts.os != "darwin" {
		t.Errorf("os = %q, want darwin", opts.os)
	}
	if opts.arch != "aarch64" {
		t.Errorf("arch = %q, want aarch64", opts.arch)
	}
}

func TestParseBuildOptsOverride(t *testing.T) {
	opts, err := parseBuildOpts([]string{"--os", "windows", "--arch", "x86_64"}, "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if opts.os != "windows" || opts.arch != "x86_64" {
		t.Errorf("got os=%q arch=%q", opts.os, opts.arch)
	}
}

func TestParseBuildOptsUnknown(t *testing.T) {
	_, err := parseBuildOpts([]string{"--nope"}, "linux", "amd64")
	if err == nil {
		t.Fatal("expected error")
	}
}
