package versions

import (
	"fmt"
	"strings"
)

// Versions holds parsed tool version mappings.
type Versions struct {
	m map[string]string
}

// Parse parses versions.env content into a Versions map.
func Parse(data string) *Versions {
	v := &Versions{m: make(map[string]string)}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, val, ok := strings.Cut(line, "="); ok {
			v.m[strings.TrimSpace(k)] = strings.TrimSpace(val)
		}
	}
	return v
}

// Get returns a version value or panics if missing.
func (v *Versions) Get(key string) string {
	val, ok := v.m[key]
	if !ok {
		panic(fmt.Sprintf("missing version key: %s", key))
	}
	return val
}

// Rewrite replaces KEY=old with KEY=val, preserving comments, blanks, and order.
// Missing keys are appended as KEY=val\n.
func Rewrite(content, key, val string) string {
	prefix := key + "="
	next := key + "=" + val
	found := false

	var b strings.Builder
	start := 0
	for start < len(content) {
		n := strings.IndexByte(content[start:], '\n')
		var line, nl string
		if n < 0 {
			line = content[start:]
			start = len(content)
		} else {
			line = content[start : start+n]
			nl = "\n"
			start += n + 1
		}
		if strings.HasPrefix(line, prefix) {
			line = next
			found = true
		}
		b.WriteString(line)
		b.WriteString(nl)
	}

	if !found {
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteByte('\n')
		}
		b.WriteString(next)
		b.WriteByte('\n')
	}
	return b.String()
}
