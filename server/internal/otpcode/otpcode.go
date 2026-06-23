// Package otpcode generates fixed-width numeric one-time codes (e.g. SMS /
// email verification codes) using a cryptographically secure RNG with no
// modulo bias.
//
// The naive pattern `randUint32() % 1_000_000` over-represents small
// numbers because 2^32 is not a multiple of 1,000,000. This package uses
// rejection sampling: read 4 bytes, treat as big-endian uint32, and
// discard values >= 4_294_000_000 (the largest multiple of 1,000,000
// that fits in 2^32). The remaining 4_294_000_000 possible values
// partition evenly into 1,000,000 buckets, so every digit appears with
// equal probability.
package otpcode

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
)

// maxMultiple is the largest multiple of 1,000,000 that fits in 2^32.
// 2^32 = 4_294_967_296; 4_294_967_296 / 1_000_000 = 4294 (rounded down),
// so 4_294 * 1_000_000 = 4_294_000_000.
const maxMultiple = uint32(1_000_000) * 4294

// Generate returns a 6-digit zero-padded numeric code as a string.
// The distribution is uniform over [000000, 999999] thanks to rejection
// sampling — no modulo bias.
func Generate() (string, error) {
	const maxAttempts = 10
	for i := 0; i < maxAttempts; i++ {
		var buf [4]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return "", fmt.Errorf("read random bytes: %w", err)
		}
		n := binary.BigEndian.Uint32(buf[:])
		if n < maxMultiple {
			return fmt.Sprintf("%06d", n%1_000_000), nil
		}
	}

	// Fallback: crypto/rand.Int — guaranteed unbiased. Used only on the
	// unlikely event that 10 consecutive 32-bit draws all fall in the
	// rejection region (probability ~ 0.0000023%).
	max := big.NewInt(int64(1_000_000))
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("rand.Int: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
