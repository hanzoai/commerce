package billing

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/account"
	"github.com/hanzoai/commerce/models/organization"
)

// subjectFor drives payer through a real request, exactly as the edge leaves it:
// the org resolved by middleware onto the ctx, and the gateway-minted identity
// headers on the wire. Every header here is stripped from client input and
// re-minted at the edge (gateway/iamauth MintedIdentityHeaders), so a test that
// sets them is modelling the trusted edge, not a forgeable client.
//
// It returns what payer resolved, so a case can assert the account rather than a
// status code.
func subjectFor(t *testing.T, org, user, claim string) string {
	t.Helper()

	var got string
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Get("/x", func(c *zip.Ctx) error {
		if org != "" {
			c.Locals("organization", &organization.Organization{Name: org})
		}
		got = payer(c).Subject()
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if user != "" {
		req.Header.Set("X-User-Id", user)
	}
	if claim != "" {
		req.Header.Set("X-Billing-Account-Id", claim)
	}
	if _, err := app.Fiber().Test(req); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return got
}

// TestPayer_CommerceAndAiAgree is the deliverable of this collapse.
//
// commerce (the grant, the top-up, the balance read) and ai (the gate, the usage
// debit) are separate repos that never call each other. The account one funds MUST
// be the account the other spends, or a customer tops up one wallet and 402s off
// another. Before this, each kept its own copy of the rule — commerce keyed on env
// allowlists, ai on the signed claim — and "they can never drift" was a comment,
// not a mechanism.
//
// Now both call account.Payer. The proof is not that the two agree on these cases:
// it is that commerce's answer is DEFINED as account.Payer's, so there is no second
// rule left to disagree. These cases pin that the header seam feeds it correctly.
func TestPayer_CommerceAndAiAgree(t *testing.T) {
	cases := []struct {
		name             string
		org, user, claim string
		want             string
	}{
		{
			name: "person in the signup org bills their own account",
			org:  "hanzo", user: "alice",
			want: "hanzo/alice",
		},
		{
			name: "person in a real org bills the org pool",
			org:  "acme", user: "bob",
			want: "acme",
		},
		{
			name: "the claim names a person in a real org — the claim wins",
			org:  "acme", user: "bob", claim: "person:acme/bob",
			want: "acme/bob",
		},
		{
			name: "the claim names the pool for a signup-org member — the claim wins",
			org:  "hanzo", user: "alice", claim: "org:hanzo",
			want: "hanzo",
		},
		{
			name: "the claim names a project — a project is a first-class payer",
			org:  "acme", user: "bob", claim: "project:acme/website",
			want: "acme/website",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := subjectFor(t, tc.org, tc.user, tc.claim)
			if got != tc.want {
				t.Fatalf("commerce subject = %q, want %q", got, tc.want)
			}
			// ai resolves the same credential through the same function. Assert the
			// values commerce feeds it produce ai's answer — the grant and the gate
			// landing on one account.
			ai := account.Payer(account.Credential{
				Owner: tc.org, Name: tc.user, Account: tc.claim,
			}).Subject()
			if got != ai {
				t.Fatalf("commerce=%q ai=%q — the grant and the gate disagree (the split-brain bug)", got, ai)
			}
		})
	}
}

// TestPayer_EnvIsInert is the tripwire for the tourniquet in our own runbook.
//
// ORG_BILLING_ORGS=hanzo was a documented billing mitigation. While commerce read
// it and ai did not, arming it would have split them apart: commerce crediting
// "hanzo" while ai debited "hanzo/alice" — funding one account and 402'ing off the
// other. Neither var is read by anything now, so the tourniquet cannot be armed.
func TestPayer_EnvIsInert(t *testing.T) {
	t.Setenv("ORG_BILLING_ORGS", "hanzo")            // would have pooled the signup org
	t.Setenv("PERSONAL_BILLING_ORGS", "acme,globex") // would have split acme per-user

	if got := subjectFor(t, "hanzo", "alice", ""); got != "hanzo/alice" {
		t.Fatalf("with hostile ORG_BILLING_ORGS: subject = %q, want %q (env must be inert)", got, "hanzo/alice")
	}
	if got := subjectFor(t, "acme", "bob", ""); got != "acme" {
		t.Fatalf("with hostile PERSONAL_BILLING_ORGS: subject = %q, want %q (env must be inert)", got, "acme")
	}
}

// TestPayer_ForeignClaimIsRefused: a claim naming ANOTHER tenant's ledger is
// discarded, not billed. IAM never mints one, so this can only fire on a mis-wired
// caller — and it must degrade to the caller's own account, never redirect a debit
// into someone else's.
func TestPayer_ForeignClaimIsRefused(t *testing.T) {
	got := subjectFor(t, "acme", "bob", "org:victim")
	if got == "victim" {
		t.Fatalf("foreign claim billed another tenant's ledger (%q) — cross-tenant debit", got)
	}
	if got != "acme" {
		t.Fatalf("foreign claim subject = %q, want fallback to own org %q", got, "acme")
	}
}

// TestPayer_FailsClosed: no org resolved means the caller cannot be attributed, so
// it must resolve to no account and let the handler 401.
//
// This also pins the panic that the pre-collapse code hid: orgBillingKey read the
// org with GetOrganization, which MustGet-PANICS when absent, making its own nil
// check unreachable. A billing handler reached without a resolved org 500'd where
// it must fail closed. Reaching this case at all requires the non-panicking read.
func TestPayer_FailsClosed(t *testing.T) {
	// A claim cannot supply a missing org — unattributable stays unattributable.
	if got := subjectFor(t, "", "alice", "org:acme"); got != "" {
		t.Fatalf("payer with no org = %q, want empty (must fail closed, never bill a guess)", got)
	}
}
