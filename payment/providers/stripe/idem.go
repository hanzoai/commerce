package stripe

import (
	"crypto/sha256"
	"encoding/hex"
)

// keyMax is Stripe's limit on the Idempotency-Key header (255 characters,
// https://docs.stripe.com/api/idempotent_requests). A longer key is refused
// before any money moves, so it is fitted here, at Stripe's boundary.
const keyMax = 255

// fitKey passes a key within the limit through unchanged and replaces a longer
// one with the hex sha256 of the whole key: deterministic, so a retry still
// de-dups, and 256 bits wide, so two keys cannot collide onto one charge.
func fitKey(key string) string {
	if len(key) <= keyMax {
		return key
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
