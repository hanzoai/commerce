package square

import (
	"strings"
	"testing"
)

// TestFitKey pins the contract fitKey exists for: every key handed to Square is
// at most 45 characters, and a key that already fits is never rewritten.
func TestFitKey(t *testing.T) {
	// The key that broke the money plane: gatewayKey("topup", org, clientUUID)
	// is 48 characters for a 5-character org, and Square rejected it outright.
	live := "topup:hanzo:" + strings.Repeat("a", 36)

	for _, tc := range []struct {
		name string
		key  string
		want string // "" means: expect a 45-char digest, not a literal
	}{
		{name: "empty", key: "", want: ""},
		{name: "short", key: "abc123", want: "abc123"},
		{name: "uuid fallback", key: "3f2504e0-4f89-11d3-9a0c-0305e82c3301", want: "3f2504e0-4f89-11d3-9a0c-0305e82c3301"},
		{name: "exactly 45", key: strings.Repeat("k", 45), want: strings.Repeat("k", 45)},
		{name: "46 is over", key: strings.Repeat("k", 46)},
		{name: "live topup key", key: live},
		{name: "sha256 hex", key: strings.Repeat("0", 64)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := fitKey(tc.key)

			if len(got) > keyMax {
				t.Fatalf("fitKey(%d chars) = %d chars, over Square's limit of %d", len(tc.key), len(got), keyMax)
			}
			if tc.want != "" || len(tc.key) <= keyMax {
				// Within the limit: byte-identical passthrough, so a key
				// already in flight keeps replaying its own charge.
				if got != tc.want {
					t.Fatalf("fitKey(%q) = %q, want unchanged %q", tc.key, got, tc.want)
				}
				return
			}
			// Over the limit: cut to exactly the limit, and NOT a prefix of the
			// original — a prefix is what collides.
			if len(got) != keyMax {
				t.Fatalf("fitKey(%q) = %d chars, want %d", tc.key, len(got), keyMax)
			}
			if strings.HasPrefix(tc.key, got) {
				t.Fatalf("fitKey(%q) = %q, which is a prefix of the input — that is truncation, not a digest", tc.key, got)
			}
		})
	}
}

// TestFitKeyDeterministic is the property idempotency depends on: the same key
// always maps to the same Square key, so a retry de-dups at Square.
func TestFitKeyDeterministic(t *testing.T) {
	key := "topup:hanzo:" + strings.Repeat("a", 36)
	if a, b := fitKey(key), fitKey(key); a != b {
		t.Fatalf("fitKey is not deterministic: %q != %q", a, b)
	}
}

// TestFitKeyDistinct is the other half: distinct money moves must not collapse
// onto one Square key, or the second is answered with the first one's payment
// and silently never happens. These inputs share a 45-character prefix, so a
// truncating implementation fails here.
func TestFitKeyDistinct(t *testing.T) {
	prefix := "topup:some-quite-long-org-slug:" + strings.Repeat("a", 20)
	a, b := fitKey(prefix+":first"), fitKey(prefix+":second")
	if a == b {
		t.Fatalf("distinct keys collided onto %q", a)
	}
}
