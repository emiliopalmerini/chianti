// Package id provides deterministic ID and slug generators used by slices.
package id

import (
	"crypto/rand"
	"io"
	"strings"
)

// italianTransliterations maps the common accented characters used in italian
// text to their ascii equivalents. Kept as a small explicit table so the
// kernel has no third-party dependencies.
var italianTransliterations = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ä': 'a', 'ã': 'a', 'å': 'a',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'ö': 'o', 'õ': 'o',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u',
	'ñ': 'n', 'ç': 'c',
	'À': 'A', 'Á': 'A', 'Â': 'A', 'Ä': 'A', 'Ã': 'A', 'Å': 'A',
	'È': 'E', 'É': 'E', 'Ê': 'E', 'Ë': 'E',
	'Ì': 'I', 'Í': 'I', 'Î': 'I', 'Ï': 'I',
	'Ò': 'O', 'Ó': 'O', 'Ô': 'O', 'Ö': 'O', 'Õ': 'O',
	'Ù': 'U', 'Ú': 'U', 'Û': 'U', 'Ü': 'U',
	'Ñ': 'N', 'Ç': 'C',
}

// Slug transliterates italian accented characters to ascii,
// collapses runs of non-alphanumeric chars into single dashes,
// and lowercases. An empty input returns "".
func Slug(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if mapped, ok := italianTransliterations[r]; ok {
			r = mapped
		}
		// ASCII lower conversion
		if r >= 'A' && r <= 'Z' {
			r = r + ('a' - 'A')
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash && b.Len() > 0 {
			b.WriteRune('-')
			prevDash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// bookingCodeAlphabet excludes visually ambiguous characters (0/O, 1/I/L, 5/S).
const bookingCodeAlphabet = "23456789ABCDEFGHJKMNPQRTUVWXYZ"

// BookingCode returns a fresh 8-character booking code using crypto/rand.
func BookingCode() string {
	return BookingCodeWithSource(rand.Reader)
}

// BookingCodeWithSource is BookingCode with an injectable random source.
func BookingCodeWithSource(r io.Reader) string {
	buf := make([]byte, 8)
	if _, err := io.ReadFull(r, buf); err != nil {
		return ""
	}
	out := make([]byte, 8)
	for i, b := range buf {
		out[i] = bookingCodeAlphabet[int(b)%len(bookingCodeAlphabet)]
	}
	return string(out)
}
