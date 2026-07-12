package billing

import "testing"

// These pin the ONE rule that keeps money coherent: the wallet a deposit lands in must
// be the wallet the LLM gate reads. Commerce and hanzoai/ai each compute it; if they
// drift by even a prefix, a customer tops up one key and spends from another.

func TestBillingSubjectFor_PersonalOrgIsPerUser(t *testing.T) {
	t.Setenv("PERSONAL_BILLING_ORGS", "hanzo")
	resetPersonalBillingOrgsForTest()

	if got, want := BillingSubjectFor("hanzo", "alice"), "hanzo/alice"; got != want {
		t.Fatalf("BillingSubjectFor = %q, want %q (a person gets a personal wallet)", got, want)
	}
}

func TestBillingSubjectFor_PooledOrgIsTheOrg(t *testing.T) {
	t.Setenv("PERSONAL_BILLING_ORGS", "hanzo")
	resetPersonalBillingOrgsForTest()

	// A real company org pools: its applications and service keys spend its balance.
	if got, want := BillingSubjectFor("acme", "bob"), "acme"; got != want {
		t.Fatalf("BillingSubjectFor = %q, want %q (a company org pools)", got, want)
	}
}

func TestBillingSubjectFor_OrgOwnedPrincipalPaysFromThePool(t *testing.T) {
	t.Setenv("PERSONAL_BILLING_ORGS", "hanzo")
	resetPersonalBillingOrgsForTest()

	// No user identity (an application / service key) => the org's own account.
	if got, want := BillingSubjectFor("hanzo", ""), "hanzo"; got != want {
		t.Fatalf("BillingSubjectFor = %q, want %q", got, want)
	}
}

func TestBillingSubjectFor_NeverDoublePrefixes(t *testing.T) {
	t.Setenv("PERSONAL_BILLING_ORGS", "hanzo")
	resetPersonalBillingOrgsForTest()

	// Callers may hand us an already-qualified id; "hanzo/hanzo/alice" would be a
	// wallet nobody funds.
	if got, want := BillingSubjectFor("hanzo", "hanzo/alice"), "hanzo/alice"; got != want {
		t.Fatalf("BillingSubjectFor = %q, want %q", got, want)
	}
}

func TestBillingSubjectFor_EmptyOrgCannotBill(t *testing.T) {
	if got := BillingSubjectFor("", "alice"); got != "" {
		t.Fatalf("BillingSubjectFor = %q, want empty (an unresolved org must not bill anyone)", got)
	}
}
