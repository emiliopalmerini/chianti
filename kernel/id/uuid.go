package id

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"sync"
	"time"
)

// NewUUID returns a fresh UUIDv7 in canonical 8-4-4-4-12 lowercase form.
// UUIDv7 packs a 48-bit unix-milliseconds prefix so generated ids sort
// monotonically, which keeps sqlite B-tree inserts local. NewUUID preserves
// lexicographic monotonicity for rapid calls in the same process.
func NewUUID() string {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[6:]); err != nil {
		return ""
	}
	ms, seq := nextSequence(uint64(time.Now().UnixMilli()), b[6], b[7])
	return formatUUID(ms, b[6:], seq)
}

// NewUUIDAt is NewUUID with an injectable clock and random source. It is
// time-sortable across distinct milliseconds but does not use the process
// monotonic sequence. Returns "" if the random reader fails; callers treat
// that as an internal error.
func NewUUIDAt(now time.Time, r io.Reader) string {
	var b [16]byte
	ms := uint64(now.UnixMilli())
	if _, err := io.ReadFull(r, b[6:]); err != nil {
		return ""
	}
	seq := uint16(b[6]&0x0f)<<8 | uint16(b[7])
	return formatUUID(ms, b[6:], seq)
}

var monotonicUUID struct {
	sync.Mutex
	lastMS  uint64
	lastSeq uint16
}

func nextSequence(ms uint64, hi, lo byte) (uint64, uint16) {
	monotonicUUID.Lock()
	defer monotonicUUID.Unlock()

	seq := uint16(hi&0x0f)<<8 | uint16(lo)
	if ms <= monotonicUUID.lastMS {
		ms = monotonicUUID.lastMS
		seq = (monotonicUUID.lastSeq + 1) & 0x0fff
		if seq == 0 {
			ms = monotonicUUID.lastMS + 1
		}
	}
	monotonicUUID.lastMS = ms
	monotonicUUID.lastSeq = seq
	return ms, seq
}

func formatUUID(ms uint64, random []byte, seq uint16) string {
	var b [16]byte
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	copy(b[6:], random)
	b[6] = 0x70 | byte(seq>>8) // version 7 and high sequence bits
	b[7] = byte(seq)
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
