package italy

import (
	"testing"
	"time"
)

func TestFiscalCode(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"RSSMRA85T10A562S", true},
		{"rssmra85t10a562s", true}, // case-insensitive
		{"RSSMRA85T10A562", false}, // too short
		{"RSSMRA85T10A562SX", false},
		{"RSSMRA85T10A562!", false}, // bad char
		{"", false},
	}
	for _, tc := range cases {
		got := ValidFiscalCode(tc.in)
		if got != tc.want {
			t.Errorf("ValidFiscalCode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestCAP(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"00100", true},
		{"20121", true},
		{"1234", false},
		{"123456", false},
		{"abcde", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := ValidCAP(tc.in); got != tc.want {
			t.Errorf("ValidCAP(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestProvince(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"MI", true},
		{"RM", true},
		{"mi", true}, // case-insensitive
		{"ZZ", false},
		{"M", false},
		{"MIL", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := ValidProvince(tc.in); got != tc.want {
			t.Errorf("ValidProvince(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestFormatDate(t *testing.T) {
	d := time.Date(2026, 1, 2, 15, 4, 0, 0, time.UTC)
	got := FormatDate(d)
	want := "2 gennaio 2026, ore 15:04"
	if got != want {
		t.Errorf("FormatDate = %q, want %q", got, want)
	}
}

func TestFormatDateOnly(t *testing.T) {
	d := time.Date(2026, 1, 2, 15, 4, 0, 0, time.UTC)
	got := FormatDateOnly(d)
	want := "2 gennaio 2026"
	if got != want {
		t.Errorf("FormatDateOnly = %q, want %q", got, want)
	}
}

func TestFormatEuroCents(t *testing.T) {
	cases := map[int]string{
		0:    "€ 0,00",
		1:    "€ 0,01",
		99:   "€ 0,99",
		150:  "€ 1,50",
		4200: "€ 42,00",
		9999: "€ 99,99",
	}
	for cents, want := range cases {
		if got := FormatEuroCents(cents); got != want {
			t.Errorf("FormatEuroCents(%d) = %q, want %q", cents, got, want)
		}
	}
}

func TestValidPhone(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"3331234567", true},          // mobile italiano
		{"+39 333 123 4567", true},    // internazionale con glifi
		{"(02) 1234.567", true},       // fisso con presentazione
		{"333-123-4567", true},        // dash
		{"  3331234567  ", true},      // trim
		{"123", false},                // troppo corto
		{"12345678901234567890123", false}, // troppo lungo
		{"abc1234567", false},         // chars invalidi
		{"333+1234567", false},        // plus non in prima posizione
		{"", false},                   // vuoto
	}
	for _, tc := range cases {
		if got := ValidPhone(tc.in); got != tc.want {
			t.Errorf("ValidPhone(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
