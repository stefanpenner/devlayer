package sysroot

import (
	"slices"
	"testing"
)

func TestOrderRealGraph(t *testing.T) {
	got, err := Order(packages())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"mbedtls", "ncurses", "zlib", "curl"}
	if !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if i, j := slices.Index(got, "zlib"), slices.Index(got, "curl"); i > j {
		t.Fatalf("zlib at %d is after curl at %d", i, j)
	}
	if i, j := slices.Index(got, "mbedtls"), slices.Index(got, "curl"); i > j {
		t.Fatalf("mbedtls at %d is after curl at %d", i, j)
	}
}

func TestOrderCycle(t *testing.T) {
	_, err := Order([]Package{
		{Name: "a", Depends: []string{"b"}},
		{Name: "b", Depends: []string{"a"}},
	})
	if err == nil {
		t.Fatal("expected a cycle error")
	}
}

func TestOrderUnknownDep(t *testing.T) {
	_, err := Order([]Package{{Name: "curl", Depends: []string{"openssl"}}})
	if err == nil {
		t.Fatal("expected an unknown dependency error")
	}
}
