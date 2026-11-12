package paypal

import (
	"strings"
	"testing"
)

// A key PayPal takes passes through unchanged, so keys already in flight keep
// replaying; a longer one maps to one deterministic key within PayPal's limit,
// and two different long keys never map to the same one.
func TestFitKey(t *testing.T) {
	short := "topup:hanzo/alice:k1"
	if got := fitKey(short); got != short {
		t.Fatalf("fitKey(%q) = %q; want it unchanged", short, got)
	}
	edge := strings.Repeat("k", keyMax)
	if got := fitKey(edge); got != edge {
		t.Fatal("a key of exactly keyMax characters must pass unchanged")
	}

	// The key a guest plan order composed on hanzo.ai/pay: 129 characters.
	long := "paypal-order-subscribe:hanzo/guest-0123456789ab:paypal-sub-order:store::plan:pro-3x:level:0:interval:month:qty:1:coupon::w1993902"
	if len(long) <= keyMax {
		t.Fatalf("fixture is %d characters; it must exceed %d", len(long), keyMax)
	}
	got := fitKey(long)
	if len(got) == 0 || len(got) > keyMax {
		t.Fatalf("fitKey(long) is %d characters; want 1..%d", len(got), keyMax)
	}
	if fitKey(long) != got {
		t.Fatal("fitKey is not deterministic, so a retry would not de-dup at PayPal")
	}
	if other := fitKey(strings.Replace(long, "pro-3x", "max-15x", 1)); other == got {
		t.Fatal("two different keys fitted to the same PayPal-Request-Id")
	}
}
