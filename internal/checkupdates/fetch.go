package checkupdates

import (
	"encoding/json"
	"strings"
)

func githubReleaseURL(repo string) string {
	return "https://api.github.com/repos/" + repo + "/releases/latest"
}

func LatestGitHub(get Get, repo string) string {
	body, err := get(githubReleaseURL(repo))
	if err != nil || len(body) == 0 {
		return ""
	}
	tag, err := ParseGitHubLatestJSON(body)
	if err != nil {
		return ""
	}
	return accept(StripPrefix(tag))
}

func LatestGitTags(get Get) string {
	var stable []string
	for _, n := range gitTagNames(get) {
		n = strings.TrimPrefix(n, "v")
		if gitStableTag.MatchString(n) {
			stable = append(stable, n)
		}
	}
	return accept(MaxSemver(stable))
}

func gitTagNames(get Get) []string {
	body, err := get("https://api.github.com/repos/git/git/tags?per_page=100")
	if err != nil || len(body) == 0 {
		return nil
	}
	names, err := parseGitHubTagsJSON(body)
	if err != nil {
		return nil
	}
	if len(names) < 100 {
		return names
	}

	page2, err := get("https://api.github.com/repos/git/git/tags?per_page=100&page=2")
	if err != nil {
		return names
	}
	more, err := parseGitHubTagsJSON(page2)
	if err != nil {
		return names
	}
	return append(names, more...)
}

func LatestGitWindows(get Get) string {
	body, err := get(githubReleaseURL("git-for-windows/git"))
	if err != nil || len(body) == 0 {
		return ""
	}
	tag, err := ParseGitHubLatestJSON(body)
	if err != nil {
		return ""
	}
	return accept(GitWindowsVersion(tag))
}

func LatestGo(get Get) string {
	body, err := get("https://go.dev/dl/?mode=json")
	if err != nil || len(body) == 0 {
		return ""
	}
	var docs []struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(body, &docs) != nil || len(docs) == 0 {
		return ""
	}
	return accept(StripPrefix(docs[0].Version))
}

func LatestZig(get Get) string {
	body, err := get("https://ziglang.org/download/index.json")
	if err != nil || len(body) == 0 {
		return ""
	}
	var index map[string]json.RawMessage
	if json.Unmarshal(body, &index) != nil {
		return ""
	}
	var vers []string
	for k := range index {
		if zigReleaseKey.MatchString(k) {
			vers = append(vers, k)
		}
	}
	return accept(MaxSemver(vers))
}

func LatestZsh(get Get) string {
	body, err := get("https://www.zsh.org/pub/")
	if err != nil || len(body) == 0 {
		return ""
	}
	return accept(MaxSemver(htmlVersions(body, zshTarball)))
}

func LatestGNU(get Get, project string) string {
	body, err := get("https://ftp.gnu.org/gnu/" + project + "/")
	if err != nil || len(body) == 0 {
		return ""
	}
	return accept(MaxSemver(htmlVersions(body, gnuTarball(project))))
}
