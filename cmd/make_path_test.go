package cmd

import "testing"

func TestBuildMakeToolDarwinUsesSystemMake(t *testing.T) {
	if got := buildMakeTool("darwin"); got != "/usr/bin/make" {
		t.Fatalf("unexpected make path: %s", got)
	}
}

func TestBuildMakeToolNonDarwinUsesMake(t *testing.T) {
	if got := buildMakeTool("linux"); got != "make" {
		t.Fatalf("unexpected make path: %s", got)
	}
}
