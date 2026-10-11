// Command hermeticgraph writes hermetic.md from bazel query.
package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// skipClass drops select machinery. The markdown names these classes.
var skipClass = map[string]bool{
	"config_setting":     true,
	"constraint_value":   true,
	"constraint_setting": true,
}

type ruleInfo struct {
	class string
	file  string
	line  int
}

type link struct {
	text string
	href string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func chooseRoot(flagRoot string) (string, error) {
	if flagRoot == "." {
		if wd := os.Getenv("BUILD_WORKING_DIRECTORY"); wd != "" {
			flagRoot = wd
		}
	}
	return filepath.Abs(flagRoot)
}

func run(args []string) error {
	fs := flag.NewFlagSet("hermeticgraph", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "workspace root")
	out := fs.String("out", "hermetic.md", "markdown output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	abs, err := chooseRoot(*root)
	if err != nil {
		return err
	}
	expr := queryExpr()
	graphText, err := bazel(abs, "graph", expr)
	if err != nil {
		return err
	}
	xmlText, err := bazel(abs, "xml", expr)
	if err != nil {
		return err
	}
	deps, err := parseGraph(bytes.NewReader(graphText))
	if err != nil {
		return err
	}
	infos, err := parseXML(bytes.NewReader(xmlText))
	if err != nil {
		return err
	}
	text, err := markdown(abs, infos, deps)
	if err != nil {
		return err
	}
	outPath := *out
	if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(abs, outPath)
	}
	return os.WriteFile(outPath, []byte(text), 0o644)
}

func queryExpr() string {
	ws := `kind("rule", //third_party/...) + kind("rule", //linux/...) + kind("rule", //macos/...) + kind("rule", //windows/...) + kind("rule", //chain/...)`
	return "let ws = " + ws + " in $ws + deps($ws, 1)"
}

func bazel(root, format, expr string) ([]byte, error) {
	args := []string{"query", "--output=" + format}
	if format == "graph" {
		args = append(args, "--nograph:factored")
	}
	args = append(args, expr)
	cmd := exec.Command("bazel", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			msg := strings.TrimSpace(string(ee.Stderr))
			return nil, fmt.Errorf("hermeticgraph: bazel query: %s", msg)
		}
		return nil, err
	}
	return out, nil
}

func parseGraph(r io.Reader) (map[string][]string, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64), 4<<20)
	seen := map[string]map[string]bool{}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.Contains(line, "->") {
			continue
		}
		left, right, ok := strings.Cut(line, "->")
		if !ok {
			continue
		}
		from, err := unquoteLabel(left)
		if err != nil {
			return nil, err
		}
		to, err := unquoteLabel(right)
		if err != nil {
			return nil, err
		}
		if seen[from] == nil {
			seen[from] = map[string]bool{}
		}
		seen[from][to] = true
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for from, tos := range seen {
		list := make([]string, 0, len(tos))
		for to := range tos {
			list = append(list, to)
		}
		sort.Strings(list)
		out[from] = list
	}
	return out, nil
}

func unquoteLabel(s string) (string, error) {
	s = strings.TrimSpace(s)
	text, err := strconv.Unquote(s)
	if err != nil {
		return "", fmt.Errorf("hermeticgraph: bad label %q", s)
	}
	return text, nil
}

func parseXML(r io.Reader) (map[string]ruleInfo, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	body = bytes.Replace(body, []byte(`version="1.1"`), []byte(`version="1.0"`), 1)
	dec := xml.NewDecoder(bytes.NewReader(body))
	out := map[string]ruleInfo{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "rule" {
			continue
		}
		info := ruleInfo{}
		name := ""
		for _, attr := range start.Attr {
			switch attr.Name.Local {
			case "class":
				info.class = attr.Value
			case "name":
				name = attr.Value
			case "location":
				file, line, err := splitLocation(attr.Value)
				if err != nil {
					return nil, err
				}
				info.file = file
				info.line = line
			}
		}
		if name != "" {
			out[name] = info
		}
	}
}

func splitLocation(loc string) (string, int, error) {
	col := strings.LastIndex(loc, ":")
	if col < 0 {
		return "", 0, fmt.Errorf("hermeticgraph: bad location %q", loc)
	}
	lineAt := strings.LastIndex(loc[:col], ":")
	if lineAt < 0 {
		return "", 0, fmt.Errorf("hermeticgraph: bad location %q", loc)
	}
	n, err := strconv.Atoi(loc[lineAt+1 : col])
	if err != nil {
		return "", 0, fmt.Errorf("hermeticgraph: bad location %q", loc)
	}
	return loc[:lineAt], n, nil
}

func topo(infos map[string]ruleInfo, deps map[string][]string) ([][]string, error) {
	nodes := map[string]bool{}
	add := func(name string) {
		info, ok := infos[name]
		if ok && skipClass[info.class] {
			return
		}
		nodes[name] = true
	}
	for name := range infos {
		add(name)
	}
	for from, tos := range deps {
		add(from)
		for _, to := range tos {
			add(to)
		}
	}
	fd := map[string][]string{}
	for name := range nodes {
		for _, to := range deps[name] {
			if nodes[to] {
				fd[name] = append(fd[name], to)
			}
		}
		sort.Strings(fd[name])
	}
	memo := map[string]int{}
	color := map[string]int{}
	var walk func(string) (int, error)
	walk = func(name string) (int, error) {
		if color[name] == 1 {
			return 0, fmt.Errorf("hermeticgraph: cycle at %s", name)
		}
		if color[name] == 2 {
			return memo[name], nil
		}
		color[name] = 1
		layer := 0
		for _, dep := range fd[name] {
			got, err := walk(dep)
			if err != nil {
				return 0, err
			}
			if got+1 > layer {
				layer = got + 1
			}
		}
		color[name] = 2
		memo[name] = layer
		return layer, nil
	}
	names := make([]string, 0, len(nodes))
	max := 0
	for name := range nodes {
		names = append(names, name)
		got, err := walk(name)
		if err != nil {
			return nil, err
		}
		if got > max {
			max = got
		}
	}
	sort.Strings(names)
	layers := make([][]string, max+1)
	for _, name := range names {
		layers[memo[name]] = append(layers[memo[name]], name)
	}
	return layers, nil
}

func markdown(root string, infos map[string]ruleInfo, deps map[string][]string) (string, error) {
	layers, err := topo(infos, deps)
	if err != nil {
		return "", err
	}
	pins := indexPins(root)
	var b strings.Builder
	b.WriteString("# Hermetic graph\n\n")
	b.WriteString("This file is the portable hermetic build.\n")
	b.WriteString("The sysroot is the output of that build.\n")
	b.WriteString("A build target, a test target, and a run target use that sysroot.\n")
	b.WriteString("`hermeticgraph` writes this file from `bazel query`.\n")
	b.WriteString("Each layer is a topological band.\n")
	b.WriteString("A child is an earlier input.\n")
	b.WriteString("The query is the rules under `third_party`, `linux`, `macos`, `windows`, and `chain`, plus one direct dependency.\n")
	b.WriteString("The generator omits `config_setting`, `constraint_value`, and `constraint_setting`.\n")
	b.WriteString("A source file has no rule class.\n")
	b.WriteString("A `@bazel_tools` label has no file in this repo.\n\n")
	for i, layer := range layers {
		fmt.Fprintf(&b, "## Layer %d\n\n", i)
		for _, name := range layer {
			info := infos[name]
			fmt.Fprintf(&b, "<a id=%q></a>\n\n", anchor(name))
			fmt.Fprintf(&b, "### `%s`\n\n", name)
			if info.class != "" {
				fmt.Fprintf(&b, "Rule `%s`.\n", info.class)
			}
			if file, ok := fileLink(root, info); ok {
				fmt.Fprintf(&b, "File [%s](%s).\n", file.text, file.href)
			} else if src, ok := sourceLink(root, name); ok {
				fmt.Fprintf(&b, "File [%s](%s).\n", src.text, src.href)
			} else if pin, ok := pins[repoOf(name)]; ok {
				fmt.Fprintf(&b, "Pin [%s](%s).\n", pin.text, pin.href)
			}
			inputs := keptDeps(name, deps, infos)
			if len(inputs) == 0 {
				b.WriteString("\n")
				continue
			}
			b.WriteString("\nInputs:\n\n")
			for _, input := range inputs {
				fmt.Fprintf(&b, "- [`%s`](#%s)\n", input, anchor(input))
			}
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

func keptDeps(name string, deps map[string][]string, infos map[string]ruleInfo) []string {
	var out []string
	for _, dep := range deps[name] {
		info, ok := infos[dep]
		if ok && skipClass[info.class] {
			continue
		}
		out = append(out, dep)
	}
	return out
}

func fileLink(root string, info ruleInfo) (link, bool) {
	if info.file == "" || info.line == 0 {
		return link{}, false
	}
	rel, err := filepath.Rel(root, info.file)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return link{}, false
	}
	rel = filepath.ToSlash(rel)
	return link{text: rel, href: fmt.Sprintf("%s#L%d", rel, info.line)}, true
}

func sourceLink(root, label string) (link, bool) {
	if !strings.HasPrefix(label, "//") {
		return link{}, false
	}
	rest := strings.TrimPrefix(label, "//")
	pkg, target, ok := strings.Cut(rest, ":")
	if !ok || target == "" || strings.Contains(target, ":") {
		return link{}, false
	}
	rel := target
	if pkg != "" {
		rel = pkg + "/" + target
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == ".." || strings.HasPrefix(rel, "../") || filepath.IsAbs(rel) {
		return link{}, false
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil || info.IsDir() {
		return link{}, false
	}
	return link{text: rel, href: rel}, true
}

func repoOf(label string) string {
	if !strings.HasPrefix(label, "@") {
		return ""
	}
	rest := strings.TrimPrefix(label, "@")
	i := strings.Index(rest, "//")
	if i < 0 {
		return ""
	}
	return rest[:i]
}

func anchor(label string) string {
	s := strings.TrimPrefix(label, "@")
	s = strings.TrimPrefix(s, "//")
	s = strings.ReplaceAll(s, "//", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	return s
}

func indexPins(root string) map[string]link {
	out := map[string]link{}
	nameRe := regexp.MustCompile(`name\s*=\s*"([^"]+)"`)
	files := []string{"tools/extensions.bzl", "MODULE.bazel", "tools/macos_sdk.bzl"}
	for _, rel := range files {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		lines := strings.Split(string(body), "\n")
		for i, line := range lines {
			m := nameRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			if _, ok := out[m[1]]; ok {
				continue
			}
			out[m[1]] = link{text: rel, href: fmt.Sprintf("%s#L%d", rel, i+1)}
		}
		for name, line := range useRepoLines(lines) {
			if _, ok := out[name]; ok {
				continue
			}
			out[name] = link{text: rel, href: fmt.Sprintf("%s#L%d", rel, line)}
		}
	}
	return out
}

func useRepoLines(lines []string) map[string]int {
	quoteRe := regexp.MustCompile(`"([^"]+)"`)
	out := map[string]int{}
	depth := 0
	active := false
	for i, line := range lines {
		if !active && strings.Contains(line, "use_repo(") {
			active = true
		}
		if !active {
			continue
		}
		for _, m := range quoteRe.FindAllStringSubmatch(line, -1) {
			if _, ok := out[m[1]]; ok {
				continue
			}
			out[m[1]] = i + 1
		}
		depth += strings.Count(line, "(") - strings.Count(line, ")")
		if depth <= 0 {
			active = false
			depth = 0
		}
	}
	return out
}
