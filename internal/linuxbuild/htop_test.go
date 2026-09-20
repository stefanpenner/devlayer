package linuxbuild

import (
	"io"
	"strings"
	"testing"
)

func TestHtopMissingVersion(t *testing.T) {
	var r recorder
	err := Htop(r.exec, r.output, map[string]string{}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "HTOP_VERSION") {
		t.Fatalf("got %v, want missing HTOP_VERSION", err)
	}
	if len(r.cmds) != 0 {
		t.Fatalf("ran commands on missing env: %v", r.cmds)
	}
}

func TestHtop(t *testing.T) {
	chdirTemp(t)
	var r recorder
	ver := "3.4.1"
	if err := Htop(r.exec, r.output, map[string]string{"HTOP_VERSION": ver}, io.Discard); err != nil {
		t.Fatal(err)
	}
	r.hasSeq(t,
		"git clone --depth 1 --branch "+ver+" https://github.com/htop-dev/htop.git",
		"htop: ./autogen.sh",
		"htop: ./configure --enable-static LDFLAGS=-static CFLAGS=-Os -DNDEBUG",
		"htop: make "+jobsArg(),
		"htop: strip htop",
		"htop: tar czf - htop",
	)
}
