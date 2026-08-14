package billing

import (
	"testing"

	"github.com/hanzoai/commerce/billing/credit"
)

// Credit is good for a year, and the request may shorten that and not extend it.
//
// The case that matters is the LAST one: `expiresIn` is client input on a
// money-in route, so a caller that names a bigger number than the policy must
// get the policy — otherwise the expiry is advisory and anyone funding an
// account can mint credit that outlives it.
func TestExpiryDays(t *testing.T) {
	for _, c := range []struct {
		name      string
		requested int
		want      int
	}{
		{"unset takes the policy", 0, credit.LifetimeDays},
		{"negative takes the policy", -30, credit.LifetimeDays},
		{"a shorter promotion is honoured", 30, 30},
		{"one day", 1, 1},
		{"the policy itself", credit.LifetimeDays, credit.LifetimeDays},
		{"longer than the policy is clamped", credit.LifetimeDays + 1, credit.LifetimeDays},
		{"ten years is clamped", 3650, credit.LifetimeDays},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := expiryDays(c.requested); got != c.want {
				t.Fatalf("expiryDays(%d) = %d, want %d", c.requested, got, c.want)
			}
		})
	}
}

// The starter grant's expiry and every other credit's are ONE value. They were
// the same number written twice; a test is what keeps them from drifting back
// apart the next time one of them is tuned.
func TestStarterCreditUsesTheOneLifetime(t *testing.T) {
	if credit.StarterCreditDays != credit.LifetimeDays {
		t.Fatalf("StarterCreditDays = %d, LifetimeDays = %d — the starter grant must not carry its own expiry",
			credit.StarterCreditDays, credit.LifetimeDays)
	}
	if credit.LifetimeDays != 365 {
		t.Fatalf("LifetimeDays = %d, want 365 — credit is good for a year", credit.LifetimeDays)
	}
}
