package organization

import "testing"

// ceiling caps every org at k for one test and restores the cap after.
func ceiling(t *testing.T, k Kind) {
	t.Helper()
	prev := Ceiling
	Ceiling = k
	t.Cleanup(func() { Ceiling = prev })
}

// Undeclared, there is no cap: the record decides.
func TestCeilingUndeclaredIsNoCap(t *testing.T) {
	if Ceiling != Live {
		t.Fatalf("Ceiling = %q with nothing declared, want %q (no cap)", Ceiling, Live)
	}
	o := Organization{}
	o.Live = true
	if o.TestMode() {
		t.Fatal("a live org is in test mode with no ceiling declared")
	}
}

// A Test ceiling puts a record marked Live in test, on every reading of the
// mode: TestMode, the Square environment, In(Live) and the Stripe token.
func TestCeilingTestCapsALiveRecord(t *testing.T) {
	ceiling(t, Test)

	o := Organization{}
	o.Live = true
	o.Stripe.Live.AccessToken = "sk_live_hostile"
	o.Stripe.Test.AccessToken = "sk_test_ok"

	if !o.TestMode() {
		t.Fatal("TestMode = false for a live record under a Test ceiling")
	}
	if got := o.SquareEnvironment(); got != "sandbox" {
		t.Fatalf("SquareEnvironment = %q, want sandbox", got)
	}
	if !o.In(Live).TestMode() {
		t.Fatal("In(Live) lifted the ceiling")
	}
	if got := o.StripeToken(); got != "sk_test_ok" {
		t.Fatalf("StripeToken = %q under a Test ceiling, want the test token", got)
	}
}

// A ceiling nobody recognises caps, never lifts.
func TestCeilingUnknownCaps(t *testing.T) {
	ceiling(t, Kind("staging"))

	o := Organization{}
	o.Live = true
	if !o.TestMode() {
		t.Fatal("an unrecognised ceiling let a live record transact live")
	}
}
