package linuxbuild

import (
	"io"
	"strings"
	"testing"
)

func TestMakeMissingVersion(t *testing.T) {
	var r recorder
	err := Make(r.exec, r.output, map[string]string{}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "MAKE_VERSION") {
		t.Fatalf("got %v, want missing MAKE_VERSION", err)
	}
}

func TestMake(t *testing.T) {
	chdirTemp(t)
	var r recorder
	ver := "4.4.1"
	dir := "make-" + ver
	if err := Make(r.exec, r.output, map[string]string{"MAKE_VERSION": ver}, io.Discard); err != nil {
		t.Fatal(err)
	}
	url := "https://ftp.gnu.org/gnu/make/make-" + ver + ".tar.gz"
	r.has(t, url)
	r.hasSeq(t,
		"curl",
		"tar",
		dir+": ./configure CFLAGS=-Os -DNDEBUG LDFLAGS=-static",
		dir+": make "+jobsArg(),
		dir+": strip make",
		dir+": tar czf - make",
	)
}
