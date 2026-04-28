package id

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"time"
)

// NewUUID returns a fresh UUIDv7 in canonical 8-4-4-4-12 lowercase form.
// UUIDv7 packs a 48-bit unix-milliseconds prefix so generated ids sort
// monotonically, which keeps sqlite B-tree inserts local.
func NewUUID() string {
	return NewUUIDAt(time.Now(), rand.Reader)
}

// NewUUIDAt is NewUUID with an injectable clock and random source. Returns
// "" if the random reader fails; callers treat that as an internal error.
func NewUUIDAt(now time.Time, r io.Reader) string {
	var b [16]byte
	ms := uint64(now.UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	if _, err := io.ReadFull(r, b[6:]); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x70 // version 7
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant

	out := make([]byte, 36)
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], b[10:16])
	return string(out)
}
