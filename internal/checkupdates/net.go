package checkupdates

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const userAgent = "devlayer-checkupdates"

// NetGet is the production Get: HTTP GET with UA and GitHub bearer token.
func NetGet(url string) ([]byte, error) {
	return httpGet(url, os.Getenv)
}

func httpGet(url string, getenv func(string) string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	if auth := githubAuth(url, getenv); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return body, nil
}

func githubAuth(url string, getenv func(string) string) string {
	if !strings.Contains(url, "api.github.com") {
		return ""
	}
	if tok := getenv("GH_TOKEN"); tok != "" {
		return "Bearer " + tok
	}
	if tok := getenv("GITHUB_TOKEN"); tok != "" {
		return "Bearer " + tok
	}
	return ""
}
