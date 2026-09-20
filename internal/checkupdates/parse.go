package checkupdates

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var (
	gitStableTag  = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	zigReleaseKey = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	zshTarball    = regexp.MustCompile(`zsh-([0-9]+(?:\.[0-9])+)\.tar\.xz`)
)

// StripPrefix removes a leading v, then jq-, then go.
func StripPrefix(tag string) string {
	tag = strings.TrimPrefix(tag, "v")
	tag = strings.TrimPrefix(tag, "jq-")
	tag = strings.TrimPrefix(tag, "go")
	return tag
}

// IsRelease is false for empty, chars outside A-Za-z0-9._-, or rc/alpha/beta/nightly/snapshot.
func IsRelease(ver string) bool {
	if ver == "" {
		return false
	}
	for _, r := range ver {
		if !releaseChar(r) {
			return false
		}
	}
	lower := strings.ToLower(ver)
	for _, bad := range []string{"rc", "alpha", "beta", "nightly", "snapshot"} {
		if strings.Contains(lower, bad) {
			return false
		}
	}
	return true
}

func releaseChar(r rune) bool {
	return r == '.' || r == '_' || r == '-' ||
		(r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		(r >= '0' && r <= '9')
}

func MaxSemver(vers []string) string {
	var best []int
	bestStr := ""
	for _, v := range vers {
		parts, ok := parseSemver(v)
		if !ok {
			continue
		}
		if bestStr == "" || semverGreater(parts, best) {
			best = parts
			bestStr = v
		}
	}
	return bestStr
}

func parseSemver(v string) ([]int, bool) {
	if v == "" {
		return nil, false
	}
	fields := strings.Split(v, ".")
	parts := make([]int, len(fields))
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			return nil, false
		}
		parts[i] = n
	}
	return parts, true
}

func semverGreater(a, b []int) bool {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av != bv {
			return av > bv
		}
	}
	return false
}

func ParseGitHubLatestJSON(body []byte) (string, error) {
	var doc struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	return doc.TagName, nil
}

// GitWindowsVersion maps v2.55.0.windows.5 → 2.55.0.5.
func GitWindowsVersion(tag string) string {
	tag = strings.TrimPrefix(tag, "v")
	return strings.Replace(tag, ".windows.", ".", 1)
}

func parseGitHubTagsJSON(body []byte) ([]string, error) {
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, err
	}
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return names, nil
}

func htmlVersions(body []byte, re *regexp.Regexp) []string {
	matches := re.FindAllSubmatch(body, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, string(m[1]))
	}
	return out
}

func gnuTarball(project string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(project) + `-([0-9]+(?:\.[0-9])+)\.tar\.gz`)
}

func accept(ver string) string {
	if IsRelease(ver) {
		return ver
	}
	return ""
}
