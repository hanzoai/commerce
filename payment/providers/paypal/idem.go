package paypal

import (
	"crypto/sha256"
	"encoding/hex"
)

// keyMax is PayPal's limit on the PayPal-Request-Id header: the Orders v2
// OpenAPI contract (paypal/paypal-rest-api-specifications,
// openapi/checkout_orders_v2.json) declares it maxLength 108. Live PayPal
// answers a longer one with 422 UNPROCESSABLE_ENTITY, issue
// INVALID_STRING_LENGTH on field PayPal-Request-Id, and opens no order — so an
// over-long key fails the payment outright. The limit is PayPal's fact and is
// enforced here, at PayPal's boundary, as Square's is in thirdparty/square.
const keyMax = 108

// fitKey fits an idempotency key into PayPal's limit.
//
// A key within the limit passes through UNCHANGED, so keys already in flight
// keep replaying the orders and captures they made.
//
// A longer key becomes the hex sha256 of the WHOLE key (64 characters). That is
// deterministic — the same key always maps to the same request id, so a retry
// still de-dups at PayPal — and 256 bits wide, so two distinct keys cannot
// collide onto one money move. Cutting the key instead would not be
// collision-free: composed keys share a prefix (op:subject:...), and PayPal
// would answer the second move with the first one's result.
func fitKey(key string) string {
	if len(key) <= keyMax {
		return key
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
