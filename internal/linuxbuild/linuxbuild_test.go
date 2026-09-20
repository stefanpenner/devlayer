package linuxbuild

import (
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func jobsArg() string {
	return "-j" + strconv.Itoa(runtime.NumCPU())
}

type recorder struct {
	cmds []string
	out  string
}

func (r *recorder) exec(dir, name string, args ...string) error {
	r.cmds = append(r.cmds, formatCmd(dir, name, args...))
	return nil
}

func (r *recorder) output(dir, name string, args ...string) (string, error) {
	r.cmds = append(r.cmds, formatCmd(dir, name, args...))
	return r.out, nil
}

func formatCmd(dir, name string, args ...string) string {
	s := strings.Join(append([]string{name}, args...), " ")
	if dir != "" {
		return dir + ": " + s
	}
	return s
}

func chdirTemp(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func (r *recorder) has(t *testing.T, sub string) {
	t.Helper()
	for _, c := range r.cmds {
		if strings.Contains(c, sub) {
			return
		}
	}
	t.Fatalf("missing %q in:\n  %s", sub, strings.Join(r.cmds, "\n  "))
}

func (r *recorder) hasSeq(t *testing.T, subs ...string) {
	t.Helper()
	i := 0
	for _, sub := range subs {
		found := -1
		for j := i; j < len(r.cmds); j++ {
			if strings.Contains(r.cmds[j], sub) {
				found = j
				break
			}
		}
		if found < 0 {
			t.Fatalf("missing %q (from index %d) in:\n  %s", sub, i, strings.Join(r.cmds, "\n  "))
		}
		i = found + 1
	}
}

func TestTools(t *testing.T) {
	want := []string{"git", "zsh", "htop", "btop", "nvim", "make"}
	got := Tools()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Tools() = %v, want %v", got, want)
	}
}

func TestRunUnknown(t *testing.T) {
	err := Run("nope", nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestRunGitMissingVersion(t *testing.T) {
	err := Run("git", map[string]string{}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "GIT_VERSION") {
		t.Fatalf("got %v, want missing GIT_VERSION", err)
	}
}
