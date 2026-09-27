package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/stefanpenner/devlayer/internal/archive"
	"github.com/stefanpenner/devlayer/internal/platform"
)

// Upgrade downloads the latest release and installs it.
func Upgrade(currentVersion string) error {
	prefix := defaultPrefix()

	p, err := platform.New(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	arch := bundleArch(p)

	// Fetch latest release info from GitHub
	fmt.Println("==> Checking for updates...")
	release, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestTag := release.TagName
	if sameVersion(latestTag, currentVersion) {
		fmt.Printf("==> Already up to date (%s)\n", currentVersion)
		return nil
	}

	fmt.Printf("==> Upgrading %s → %s\n", currentVersion, latestTag)
	if notes := strings.TrimSpace(release.Body); notes != "" {
		fmt.Println()
		fmt.Println(notes)
		fmt.Println()
	}

	// Download the bundle
	assetName := fmt.Sprintf("devlayer-%s-%s.%s", p.OS, arch, p.BundleExt)
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			// Use API URL for private repos (browser_download_url doesn't work with token auth)
			if ghToken() != "" && asset.URL != "" {
				downloadURL = asset.URL
			} else {
				downloadURL = asset.BrowserDownloadURL
			}
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no release asset found for %s/%s (%s)", p.OS, arch, assetName)
	}

	fmt.Printf("==> Downloading %s...\n", assetName)
	tmp, err := os.CreateTemp("", "devlayer-upgrade-*."+p.BundleExt)
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	resp, err := authedGet(downloadURL, "application/octet-stream")
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	size, err := io.Copy(tmp, resp.Body)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	tmp.Close()

	fmt.Printf("==> Downloaded %.0f MB\n", float64(size)/1024/1024)

	// Extract to prefix
	fmt.Printf("==> Installing to %s...\n", prefix)
	if err := os.MkdirAll(prefix, 0755); err != nil {
		return err
	}

	if p.BundleExt == "zip" {
		err = archive.ExtractZip(tmp.Name(), prefix)
	} else {
		err = archive.ExtractTarGz(tmp.Name(), prefix)
	}
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	fmt.Printf("==> Upgraded to %s\n", latestTag)
	return nil
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Body    string    `json:"body"`
	Assets  []ghAsset `json:"assets"`
}

func sameVersion(a, b string) bool {
	return strings.TrimPrefix(a, "v") == strings.TrimPrefix(b, "v")
}

func formatNotice(current, latest, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "==> New version %s (you have %s)\n    run: devlayer upgrade\n", latest, current)
	if strings.TrimSpace(body) != "" {
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(body))
		b.WriteString("\n")
	}
	return b.String()
}

type ghAsset struct {
	Name               string `json:"name"`
	URL                string `json:"url"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func ghToken() string {
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t
	}
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return ""
}

func authedGet(url, accept string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	if token := ghToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return http.DefaultClient.Do(req)
}

func getLatestRelease() (*ghRelease, error) {
	resp, err := authedGet("https://api.github.com/repos/stefanpenner/devlayer/releases/latest", "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}
