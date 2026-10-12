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

func TestParseBuildOptsArchFlagUsesZigNames(t *testing.T) {
	opts, err := parseBuildOpts([]string{"--os", "darwin", "--arch", "arm64"}, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if opts.arch != "aarch64" {
		t.Fatalf("arch = %q, want aarch64", opts.arch)
	}
	opts, err = parseBuildOpts([]string{"--os", "linux", "--arch", "amd64"}, "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if opts.arch != "x86_64" {
		t.Fatalf("arch = %q, want x86_64", opts.arch)
	}
}

func TestParseBuildOptsUnknown(t *testing.T) {
	_, err := parseBuildOpts([]string{"--nope"}, "linux", "amd64")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBundleSelfName(t *testing.T) {
	tests := []struct {
		host, target, name string
		copy               bool
	}{
		{host: "linux", target: "windows", copy: false},
		{host: "linux", target: "darwin", copy: false},
		{host: "windows", target: "windows", name: "devlayer.exe", copy: true},
		{host: "darwin", target: "darwin", name: "devlayer", copy: true},
		{host: "linux", target: "linux", name: "devlayer", copy: true},
	}
	for _, tt := range tests {
		name, copy := bundleSelfName(tt.host, tt.target)
		if copy != tt.copy || name != tt.name {
			t.Errorf("bundleSelfName(%q, %q) = %q, %v; want %q, %v", tt.host, tt.target, name, copy, tt.name, tt.copy)
		}
	}
}
