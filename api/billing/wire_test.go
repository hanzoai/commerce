package billing

import "testing"

// wireReference must survive a bank memo field: uppercase [A-Z0-9-] only, and
// distinct payers must stay distinct after the trip (reconciliation depends on
// recovering the payer from exactly this alphabet).
func TestWireReference(t *testing.T) {
	cases := []struct{ payer, want string }{
		{"hanzo", "TOPUP-HANZO"},
		{"hanzo/z", "TOPUP-HANZO-Z"},
		{"acme-labs/alice.b", "TOPUP-ACME-LABS-ALICE-B"},
		{"org_1", "TOPUP-ORG-1"},
	}
	for _, tc := range cases {
		if got := wireReference(tc.payer); got != tc.want {
			t.Errorf("wireReference(%q) = %q, want %q", tc.payer, got, tc.want)
		}
	}
}
