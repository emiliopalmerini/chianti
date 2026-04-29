// Package config exposes small primitives shared by every consumer site's
// own Config: env reading with default, 32-byte random key generation,
// and parsing of the ADMIN_SEEDS env-var format.
//
// Each site keeps its own Config struct, Load, and Validate; only the
// helpers live here.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"strings"
)

// GetEnv returns the value of key from the environment, or def if the
// variable is unset or set to the empty string.
func GetEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// RandomKey returns a 32-byte random key, base64-encoded with stdlib
// StdEncoding (44 chars). Suitable as a CSRF key, JWT signing key, or
// session secret.
func RandomKey() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return base64.StdEncoding.EncodeToString(buf)
}

// AdminSeed is one entry of the ADMIN_SEEDS env var. The expected env
// format is "user1:email1:pw1,user2:email2:pw2".
type AdminSeed struct {
	Username string
	Email    string
	Password string
}

// ParseAdminSeeds parses raw into AdminSeed entries. Empty raw returns nil.
// Malformed entries (not exactly 3 colon-separated parts) are silently
// skipped; the caller's Validate is responsible for failing fast if zero
// seeds is unacceptable.
func ParseAdminSeeds(raw string) []AdminSeed {
	if raw == "" {
		return nil
	}
	var out []AdminSeed
	for entry := range strings.SplitSeq(raw, ",") {
		parts := strings.Split(entry, ":")
		if len(parts) != 3 {
			continue
		}
		out = append(out, AdminSeed{
			Username: parts[0],
			Email:    parts[1],
			Password: parts[2],
		})
	}
	return out
}
