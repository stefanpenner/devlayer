// Package sums is the sha256 pin file for incoming tool archives.
// No pin → the download is refused.
package sums

import (
	_ "embed"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
)

//go:embed checksums.sha256
var rawPins string

var (
	loadOnce sync.Once
	loaded   map[string]string
	loadErr  error
)

// Parse reads "sha256  url" lines. Blank lines and # comments are ignored.
func Parse(text string) (map[string]string, error) {
	pins := make(map[string]string)
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sha, url, ok := strings.Cut(line, "  ")
		if !ok {
			return nil, fmt.Errorf("bad checksum line: %s", line)
		}
		sha = strings.ToLower(strings.TrimSpace(sha))
		url = strings.TrimSpace(url)
		if strings.ContainsAny(url, " \t") {
			return nil, fmt.Errorf("bad checksum url: %s", line)
		}
		if err := Valid(sha); err != nil {
			return nil, fmt.Errorf("pin %s: %w", url, err)
		}
		if prev, exists := pins[url]; exists && prev != sha {
			return nil, fmt.Errorf("duplicate pin for %s", url)
		}
		pins[url] = sha
	}
	return pins, nil
}

// Valid reports whether sum is 64 hex chars.
func Valid(sum string) error {
	sum = strings.ToLower(strings.TrimSpace(sum))
	if len(sum) != 64 {
		return fmt.Errorf("sha256 must be 64 hex chars")
	}
	if _, err := hex.DecodeString(sum); err != nil {
		return fmt.Errorf("sha256: %w", err)
	}
	return nil
}

// Pins returns the embedded pin file. The map is shared; do not mutate it.
func Pins() (map[string]string, error) {
	loadOnce.Do(func() {
		loaded, loadErr = Parse(rawPins)
	})
	return loaded, loadErr
}

// Must returns the sha256 for url or an error when the pin is missing.
func Must(url string) (string, error) {
	pins, err := Pins()
	if err != nil {
		return "", err
	}
	sum, ok := pins[url]
	if !ok || sum == "" {
		return "", fmt.Errorf("no sha256 pin for %s", url)
	}
	return sum, nil
}
