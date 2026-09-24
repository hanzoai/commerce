package processor

import "testing"

// Settled is the one definition of "money was actually taken", shared by the
// charge path and the inbound webhook path. This pins the contract directly, so
// a change to it is a deliberate act rather than a side effect.
func TestSettled(t *testing.T) {
	for _, tc := range []struct {
		status string
		want   bool
	}{
		{"COMPLETED", true},
		{"CAPTURED", true},
		{"completed", true}, // processors disagree on casing
		{" COMPLETED ", true},

		// APPROVED is an authorization hold: reserved, not taken. It can still be
		// voided or expire, and a card-verification pre-auth is exactly that shape
		// — authorized, then deliberately voided. Crediting it mints balance
		// against money that may never arrive.
		{"APPROVED", false},
		{"approved", false},

		{"PENDING", false},
		{"CANCELED", false},
		{"FAILED", false},
		{"", false},
		{"nonsense", false},
	} {
		if got := Settled(tc.status); got != tc.want {
			t.Errorf("Settled(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}
