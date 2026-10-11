package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGraphEdges(t *testing.T) {
	in := `
digraph mygraph {
  node [shape=box];
  "//third_party/htop:htop"
  "//third_party/htop:htop" -> "//third_party/ncurses:ncurses"
  "//third_party/htop:htop" -> "@htop_src//:all_srcs"
  "//third_party/htop:htop" -> "//third_party/ncurses:ncurses"
}
`
	got, err := parseGraph(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	deps := got["//third_party/htop:htop"]
	if len(deps) != 2 || deps[0] != "//third_party/ncurses:ncurses" || deps[1] != "@htop_src//:all_srcs" {
		t.Fatalf("%v", deps)
	}
}

func TestParseXMLRule(t *testing.T) {
	in := `<?xml version="1.1" encoding="UTF-8" standalone="no"?>
<query version="2">
  <rule class="configure_make" location="/repo/third_party/htop/BUILD.bazel:4:15" name="//third_party/htop:htop">
    <string name="name" value="htop"/>
    <rule-input name="//third_party/ncurses:ncurses"/>
  </rule>
</query>
`
	got, err := parseXML(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	info := got["//third_party/htop:htop"]
	if info.class != "configure_make" || info.line != 4 || !strings.HasSuffix(info.file, "third_party/htop/BUILD.bazel") {
		t.Fatalf("%+v", info)
	}
}

func TestLayersAreTopological(t *testing.T) {
	infos := map[string]ruleInfo{
		"//third_party/htop:htop":       {class: "configure_make"},
		"//third_party/ncurses:ncurses": {class: "configure_make"},
		"@ncurses_src//:all_srcs":       {class: "filegroup"},
		"//tools:is_linux":              {class: "config_setting"},
	}
	deps := map[string][]string{
		"//third_party/htop:htop":       {"//third_party/ncurses:ncurses", "//tools:is_linux", "@htop_src//:all_srcs"},
		"//third_party/ncurses:ncurses": {"@ncurses_src//:all_srcs"},
	}
	layers, err := topo(infos, deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 3 {
		t.Fatalf("layers %d %v", len(layers), layers)
	}
	if layers[0][0] != "@htop_src//:all_srcs" || layers[0][1] != "@ncurses_src//:all_srcs" {
		t.Fatalf("layer 0 %v", layers[0])
	}
	if len(layers[1]) != 1 || layers[1][0] != "//third_party/ncurses:ncurses" {
		t.Fatalf("layer 1 %v", layers[1])
	}
	if len(layers[2]) != 1 || layers[2][0] != "//third_party/htop:htop" {
		t.Fatalf("layer 2 %v", layers[2])
	}
}

func TestTopoRejectsACycle(t *testing.T) {
	infos := map[string]ruleInfo{
		"//a:a": {class: "genrule"},
		"//b:b": {class: "genrule"},
	}
	deps := map[string][]string{
		"//a:a": {"//b:b"},
		"//b:b": {"//a:a"},
	}
	_, err := topo(infos, deps)
	if err == nil {
		t.Fatal("cycle returned nil")
	}
}

func TestChooseRootUsesBuildWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BUILD_WORKING_DIRECTORY", dir)
	got, err := chooseRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestChooseRootKeepsAnExplicitPath(t *testing.T) {
	t.Setenv("BUILD_WORKING_DIRECTORY", t.TempDir())
	other := t.TempDir()
	got, err := chooseRoot(other)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(other)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestMarkdownLinksASourceFile(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "macos")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "archive.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	text, err := markdown(dir, map[string]ruleInfo{
		"//macos:archive.sh": {},
	}, map[string][]string{})
	if err != nil {
		t.Fatal(err)
	}
	want := "[macos/archive.sh](macos/archive.sh)"
	if !strings.Contains(text, want) {
		t.Fatalf("missing %s\n%s", want, text)
	}
	if strings.Contains(text, "Rule `") {
		t.Fatal("source file gained a rule class")
	}
}

func TestMarkdownLinksUseRepoNames(t *testing.T) {
	dir := t.TempDir()
	body := "use_repo(\n    tool_repos,\n    \"age_darwin_arm64\",\n)\n"
	if err := os.WriteFile(filepath.Join(dir, "MODULE.bazel"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	text, err := markdown(dir, map[string]ruleInfo{
		"@age_darwin_arm64//:age": {class: "filegroup", file: "/cache/external/age/BUILD.bazel", line: 1},
	}, map[string][]string{})
	if err != nil {
		t.Fatal(err)
	}
	want := "[MODULE.bazel](MODULE.bazel#L3)"
	if !strings.Contains(text, want) {
		t.Fatalf("missing %s\n%s", want, text)
	}
	if strings.Contains(text, "/cache/external") {
		t.Fatal("cache path leaked")
	}
}

func TestMarkdownLinksTheRule(t *testing.T) {
	dir := t.TempDir()
	ext := filepath.Join(dir, "tools")
	if err := os.MkdirAll(ext, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "load(\"x\")\n\n_http_archive(sums,\n    name = \"htop_src\",\n)\n"
	if err := os.WriteFile(filepath.Join(ext, "extensions.bzl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	infos := map[string]ruleInfo{
		"//third_party/htop:htop": {class: "configure_make", file: filepath.Join(dir, "third_party/htop/BUILD.bazel"), line: 4},
		"@htop_src//:all_srcs":    {class: "filegroup", file: "/cache/external/htop_src/BUILD.bazel", line: 1},
	}
	deps := map[string][]string{
		"//third_party/htop:htop": {"@htop_src//:all_srcs"},
	}
	text, err := markdown(dir, infos, deps)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"## Layer 0",
		"## Layer 1",
		"`filegroup`",
		"[tools/extensions.bzl](tools/extensions.bzl#L4)",
		"`configure_make`",
		"[third_party/htop/BUILD.bazel](third_party/htop/BUILD.bazel#L4)",
		"config_setting",
		"constraint_value",
		"constraint_setting",
		"A `@bazel_tools` label has no file in this repo.",
		"A source file has no rule class.",
		"This file is the portable hermetic build.",
		"The sysroot is the output of that build.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s\n%s", want, text)
		}
	}
	if strings.Contains(text, "/cache/external") {
		t.Fatal("cache path leaked")
	}
}
