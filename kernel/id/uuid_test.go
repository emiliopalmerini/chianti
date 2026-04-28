package id_test

import (
	"strings"
	"testing"
	"time"

	"github.com/emiliopalmerini/chianti/kernel/id"
)

func TestNewUUIDFormat(t *testing.T) {
	u := id.NewUUID()
	if len(u) != 36 {
		t.Fatalf("len = %d, want 36: %q", len(u), u)
	}
	for i, dash := range []int{8, 13, 18, 23} {
		if u[dash] != '-' {
			t.Errorf("dash %d at index %d: %q", i, dash, u)
		}
	}
	if u[14] != '7' {
		t.Errorf("version nibble = %q, want '7'", u[14])
	}
	if c := u[19]; c != '8' && c != '9' && c != 'a' && c != 'b' {
		t.Errorf("variant nibble = %q, want 8/9/a/b", c)
	}
}

func TestUUIDMonotonic(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	t1 := t0.Add(time.Millisecond)
	u0 := id.NewUUIDAt(t0, strings.NewReader("\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"))
	u1 := id.NewUUIDAt(t1, strings.NewReader("\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"))
	if !(u0 < u1) {
		t.Errorf("expected %q < %q", u0, u1)
	}
}
