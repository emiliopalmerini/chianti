// Package italy provides Italian-specific field validators used by
// publicform and any future slice that needs to validate fiscal codes,
// postal codes, or province codes. The rules are syntactic only: a fiscal
// code is accepted if it has the right shape, not if it resolves to a real
// registered person.
package italy

import (
	"fmt"
	"strings"
	"time"
)

var italianMonths = [12]string{
	"gennaio", "febbraio", "marzo", "aprile", "maggio", "giugno",
	"luglio", "agosto", "settembre", "ottobre", "novembre", "dicembre",
}

// FormatDate formats a time.Time as "2 gennaio 2006, ore 15:04".
func FormatDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d, ore %02d:%02d",
		t.Day(), italianMonths[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}

// FormatDateOnly formats a time.Time as "2 gennaio 2006" (no time component).
func FormatDateOnly(t time.Time) string {
	return fmt.Sprintf("%d %s %d",
		t.Day(), italianMonths[t.Month()-1], t.Year())
}

// ValidFiscalCode reports whether s is a syntactically valid Italian
// codice fiscale: 16 alphanumeric characters. Case-insensitive.
func ValidFiscalCode(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9') && !(r >= 'A' && r <= 'Z') && !(r >= 'a' && r <= 'z') {
			return false
		}
	}
	return true
}

// ValidCAP reports whether s is a 5-digit Italian postal code.
func ValidCAP(s string) bool {
	if len(s) != 5 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// italianProvinces is the closed set of Italian province codes (ISO 3166-2:IT
// minus the autonomous provinces special-cased below). 110 entries.
var italianProvinces = map[string]struct{}{
	"AG": {}, "AL": {}, "AN": {}, "AO": {}, "AR": {}, "AP": {}, "AT": {}, "AV": {},
	"BA": {}, "BT": {}, "BL": {}, "BN": {}, "BG": {}, "BI": {}, "BO": {}, "BZ": {},
	"BS": {}, "BR": {}, "CA": {}, "CL": {}, "CB": {}, "CI": {}, "CE": {}, "CT": {},
	"CZ": {}, "CH": {}, "CO": {}, "CS": {}, "CR": {}, "KR": {}, "CN": {}, "EN": {},
	"FM": {}, "FE": {}, "FI": {}, "FG": {}, "FC": {}, "FR": {}, "GE": {}, "GO": {},
	"GR": {}, "IM": {}, "IS": {}, "SP": {}, "LT": {}, "LE": {}, "LC": {}, "LI": {},
	"LO": {}, "LU": {}, "MC": {}, "MN": {}, "MS": {}, "MT": {}, "VS": {}, "ME": {},
	"MI": {}, "MO": {}, "MB": {}, "NA": {}, "NO": {}, "NU": {}, "OG": {}, "OT": {},
	"OR": {}, "PD": {}, "PA": {}, "PR": {}, "PV": {}, "PG": {}, "PU": {}, "PE": {},
	"PC": {}, "PI": {}, "PT": {}, "PN": {}, "PZ": {}, "PO": {}, "RG": {}, "RA": {},
	"RC": {}, "RE": {}, "RI": {}, "RN": {}, "RM": {}, "RO": {}, "SA": {}, "SS": {},
	"SV": {}, "SI": {}, "SR": {}, "SO": {}, "TA": {}, "TE": {}, "TR": {}, "TO": {},
	"TP": {}, "TN": {}, "TV": {}, "TS": {}, "UD": {}, "VA": {}, "VE": {}, "VB": {},
	"VC": {}, "VR": {}, "VV": {}, "VI": {}, "VT": {},
}

// FormatEuroCents formats an amount in cents as Italian currency, e.g.
// 4200 → "€ 42,00", 150 → "€ 1,50", 0 → "€ 0,00".
func FormatEuroCents(cents int) string {
	euros := cents / 100
	remainder := cents % 100
	if remainder < 0 {
		remainder = -remainder
	}
	return fmt.Sprintf("€ %d,%02d", euros, remainder)
}

// ValidPhone reports whether s looks like a plausible phone number: 6–20
// characters after trimming, composed only of digits and the usual
// presentation glyphs (space, dot, dash, parentheses, leading plus). Syntactic
// only; it does not validate country codes or subscriber numbering plans.
func ValidPhone(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 6 || len(s) > 20 {
		return false
	}
	digits := 0
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+':
			if i != 0 {
				return false
			}
		case r == ' ', r == '.', r == '-', r == '(', r == ')':
			// allowed presentation glyph
		default:
			return false
		}
	}
	return digits >= 6
}

// ValidProvince reports whether s is a two-letter Italian province code.
// Case-insensitive.
func ValidProvince(s string) bool {
	if len(s) != 2 {
		return false
	}
	_, ok := italianProvinces[strings.ToUpper(s)]
	return ok
}
