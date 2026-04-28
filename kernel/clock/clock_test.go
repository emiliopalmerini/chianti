package clock_test

import (
	"testing"
	"time"

	"github.com/emiliopalmerini/chianti/kernel/clock"
)

func TestSystemReturnsUTC(t *testing.T) {
	now := clock.System().Now()
	if now.Location() != time.UTC {
		t.Errorf("System().Now() location = %v, want UTC", now.Location())
	}
}

func TestFixedReturnsExactValue(t *testing.T) {
	want := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	got := clock.Fixed(want).Now()
	if !got.Equal(want) {
		t.Errorf("Fixed(t).Now() = %v, want %v", got, want)
	}
}

func TestFixedIsImmutable(t *testing.T) {
	c := clock.Fixed(time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC))
	a := c.Now()
	b := c.Now()
	if !a.Equal(b) {
		t.Errorf("Fixed clock returned different values: %v vs %v", a, b)
	}
}
