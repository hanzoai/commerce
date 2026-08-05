package referrer

import "testing"

// A referral code must be unique on sight, because the create handler reissues
// any code already taken in the namespace. A fixture that draws from a small
// dictionary satisfies "looks like a code" and not that, which is how the
// api/referrer spec asserting the server kept the code it was sent came to fail
// about one run in twenty for reasons entirely on the client side.
func TestNewCodeIsUnique(t *testing.T) {
	const n = 10000

	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		code := NewCode()

		if len(code) != 8 {
			t.Fatalf("NewCode() = %q, want 8 characters", code)
		}
		for _, r := range code {
			if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
				t.Fatalf("NewCode() = %q, want uppercase alphanumerics only", code)
			}
		}
		if seen[code] {
			t.Fatalf("NewCode() repeated %q within %d draws", code, n)
		}
		seen[code] = true
	}
}
