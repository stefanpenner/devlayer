package deadstrip

import (
	"strings"
	"testing"
)

func TestCheckRejectsHostIconv(t *testing.T) {
	in := machoFixture([]string{"/usr/lib/libSystem.B.dylib", "/usr/lib/libiconv.2.dylib"}, []int{1, 2}, false)
	if err := Check(in); err == nil {
		t.Fatal("accepted libiconv")
	}
}

func TestCheckRejectsABinaryWithNoPolicy(t *testing.T) {
	in := machoFixture([]string{
		"/usr/lib/libSystem.B.dylib",
		"/usr/lib/libresolv.9.dylib",
		"/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation",
	}, []int{1, 2, 3}, false)
	err := Check(in)
	if err == nil || !strings.Contains(err.Error(), "no policy") {
		t.Fatalf("got %v", err)
	}
}

func TestCheckIgnoresOtherFiles(t *testing.T) {
	if err := Check([]byte("not a binary")); err != nil {
		t.Fatal(err)
	}
}
