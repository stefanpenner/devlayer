package binaries

import (
	"fmt"
	"sort"
	"strings"

	"github.com/stefanpenner/devlayer/internal/platform"
	"github.com/stefanpenner/devlayer/internal/versions"
)

// Status returns the final HTTP status for a URL (follow redirects).
type Status func(url string) (int, error)

// OSArch is a probe target.
type OSArch struct {
	OS, Arch string
}

func DefaultProbeTargets() []OSArch {
	return []OSArch{
		{"linux", "x86_64"},
		{"linux", "aarch64"},
		{"darwin", "arm64"},
		{"windows", "x86_64"},
	}
}

// Probe HEADs each URL. Fails if any status is not 200.
func Probe(urls map[string]string, status Status) error {
	names := make([]string, 0, len(urls))
	for n := range urls {
		names = append(names, n)
	}
	sort.Strings(names)

	var fails []string
	for _, name := range names {
		code, err := status(urls[name])
		if err != nil {
			fails = append(fails, name+": "+err.Error())
			continue
		}
		if code != 200 {
			fails = append(fails, fmt.Sprintf("%s: HTTP %d", name, code))
		}
	}
	if len(fails) == 0 {
		return nil
	}
	return fmt.Errorf("probe failed:\n  %s", strings.Join(fails, "\n  "))
}

// ProbeTargets lists URLs for each OS/arch and probes them.
func ProbeTargets(vers *versions.Versions, status Status, targets []OSArch) error {
	for _, t := range targets {
		p, err := platform.New(t.OS, t.Arch)
		if err != nil {
			return err
		}
		urls, err := URLs(p, vers, nil)
		if err != nil {
			return err
		}
		if err := Probe(urls, status); err != nil {
			return fmt.Errorf("%s/%s: %w", t.OS, t.Arch, err)
		}
	}
	return nil
}
