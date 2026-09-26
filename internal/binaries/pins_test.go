package binaries

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stefanpenner/devlayer/internal/sums"
	"github.com/stefanpenner/devlayer/internal/versions"
)

func TestSourceArchiveURLs(t *testing.T) {
	got := SourceArchiveURLs(testVers())
	want := map[string]string{
		"btop":    "https://github.com/aristocratos/btop/archive/refs/tags/v1.4.7.tar.gz",
		"htop":    "https://github.com/htop-dev/htop/archive/refs/tags/3.5.3.tar.gz",
		"make":    "https://ftp.gnu.org/gnu/make/make-4.4.1.tar.gz",
		"ncurses": "https://ftp.gnu.org/gnu/ncurses/ncurses-6.6.tar.gz",
		"git":     "https://mirrors.edge.kernel.org/pub/software/scm/git/git-2.55.0.tar.xz",
		"zsh":     "https://www.zsh.org/pub/zsh-5.9.2.tar.xz",
	}
	if len(got) != len(want) {
		t.Fatalf("len %d want %d (%v)", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s\n  got  %s\n  want %s", k, got[k], v)
		}
	}
}

func TestAllURLsPinned(t *testing.T) {
	vers := versions.Parse(string(readRepoFile(t, "versions.env")))
	urls, err := AllURLs(vers)
	if err != nil {
		t.Fatal(err)
	}
	pins, err := sums.Pins()
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]struct{}{}
	var missing []string
	for _, u := range urls {
		want[u] = struct{}{}
		if _, ok := pins[u]; !ok {
			missing = append(missing, u)
			fmt.Println("PINURL " + u)
		}
	}
	var extra []string
	for u := range pins {
		if _, ok := want[u]; !ok {
			extra = append(extra, u)
		}
	}
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("missing %d pins, extra %d\nextra:\n%s", len(missing), len(extra), strings.Join(extra, "\n"))
	}
}

func readRepoFile(t *testing.T, rel string) []byte {
	t.Helper()
	var cands []string
	if src := os.Getenv("TEST_SRCDIR"); src != "" {
		ws := os.Getenv("TEST_WORKSPACE")
		if ws == "" {
			ws = "_main"
		}
		cands = append(cands, filepath.Join(src, ws, rel))
	}
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			cands = append(cands, filepath.Join(dir, rel))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	for _, p := range cands {
		b, err := os.ReadFile(p)
		if err == nil {
			return b
		}
	}
	t.Fatalf("could not read %s (tried %v)", rel, cands)
	return nil
}
