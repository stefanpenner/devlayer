package checkupdates

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubAuth(t *testing.T) {
	getenv := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	tests := []struct {
		name string
		url  string
		env  map[string]string
		want string
	}{
		{
			name: "non-github",
			url:  "https://go.dev/dl/?mode=json",
			env:  map[string]string{"GH_TOKEN": "x"},
			want: "",
		},
		{
			name: "GH_TOKEN wins",
			url:  "https://api.github.com/repos/junegunn/fzf/releases/latest",
			env:  map[string]string{"GH_TOKEN": "a", "GITHUB_TOKEN": "b"},
			want: "Bearer a",
		},
		{
			name: "GITHUB_TOKEN fallback",
			url:  "https://api.github.com/repos/junegunn/fzf/releases/latest",
			env:  map[string]string{"GITHUB_TOKEN": "b"},
			want: "Bearer b",
		},
		{
			name: "no token",
			url:  "https://api.github.com/repos/junegunn/fzf/releases/latest",
			env:  nil,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := githubAuth(tt.url, getenv(tt.env)); got != tt.want {
				t.Errorf("githubAuth = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHTTPGetUserAgentAndStatus(t *testing.T) {
	var ua string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		if r.URL.Path == "/missing" {
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		io.WriteString(w, "ok")
	}))
	defer s.Close()

	body, err := httpGet(s.URL, osGetenv(nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q", body)
	}
	if ua != userAgent {
		t.Errorf("User-Agent = %q, want %q", ua, userAgent)
	}

	_, err = httpGet(s.URL+"/missing", osGetenv(nil))
	if err == nil || err.Error() != fmt.Sprintf("HTTP %d", http.StatusNotFound) {
		t.Errorf("404 err = %v", err)
	}
}

func osGetenv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}
