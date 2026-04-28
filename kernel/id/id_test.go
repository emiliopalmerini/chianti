package id_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/emiliopalmerini/chianti/kernel/id"
)

func TestSlugItalianAccents(t *testing.T) {
	cases := map[string]string{
		"Tòrneo dei Bambini":     "torneo-dei-bambini",
		"Città di Milano":        "citta-di-milano",
		"Caffè & Latte":          "caffe-latte",
		"Workshop d'estate 2026": "workshop-d-estate-2026",
		"   multiple   spaces  ": "multiple-spaces",
		"":                       "",
	}
	for in, want := range cases {
		if got := id.Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBookingCodeFormat(t *testing.T) {
	const alphabet = "23456789ABCDEFGHJKMNPQRTUVWXYZ"
	code := id.BookingCode()
	if len(code) != 8 {
		t.Fatalf("len = %d", len(code))
	}
	for _, c := range code {
		if !strings.ContainsRune(alphabet, c) {
			t.Errorf("char %q not in alphabet", c)
		}
	}
}

func TestBookingCodeWithSourceDeterministic(t *testing.T) {
	src := bytes.NewReader(make([]byte, 16))
	a := id.BookingCodeWithSource(src)
	src2 := bytes.NewReader(make([]byte, 16))
	b := id.BookingCodeWithSource(src2)
	if a != b {
		t.Errorf("expected deterministic output, got %q vs %q", a, b)
	}
}
