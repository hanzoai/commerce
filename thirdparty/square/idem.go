package square

import (
	"crypto/sha256"
	"encoding/hex"
)

// keyMax is Square's hard limit on idempotency_key. A longer key is rejected
// outright — VALUE_TOO_LONG / INVALID_REQUEST_ERROR — before any money moves,
// so the charge fails rather than double-charging. The limit is Square's fact
// and is enforced here, at Square's boundary: callers compose keys for their
// own reasons (see api/billing gatewayKey) and other processors have their own
// limits, so no caller should have to know this number.
const keyMax = 45

// refMax is Square's limit on a customer's reference_id. A longer reference
// fails the create, so it is fitted the same way a key is.
const refMax = 100

// fitKey fits an idempotency key into Square's limit.
//
// A key already within the limit passes through UNCHANGED, so keys in flight
// keep replaying the charges they already made.
//
// A longer key becomes the hex sha256 of the WHOLE key, cut to the limit. That
// is deterministic — the same key always maps to the same Square key, so
// idempotency survives the mapping and a retry still de-dups — and it is 180
// bits wide, so two distinct keys cannot collide onto one charge.
//
// Cutting the key itself would be deterministic but NOT collision-free:
// composed keys share a prefix (op:subject:...), so two different money moves
// can cut to the same key, and Square would answer the second with the first
// one's payment — a charge that silently never happens.
func fitKey(key string) string {
	return fit(key, keyMax)
}

// fit returns s unchanged when it is within limit, else the hex sha256 of the
// whole of s, cut to limit.
func fit(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	sum := sha256.Sum256([]byte(s))
	d := hex.EncodeToString(sum[:])
	return d[:min(len(d), limit)]
}
