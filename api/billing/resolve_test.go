package billing

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/account"
	"github.com/hanzoai/commerce/models/organization"
)

// ctxFor builds a request context exactly as the edge leaves it: the org resolved
// by middleware, and the gateway-minted identity headers. Every header here is
// stripped from client input and re-minted at the edge (gateway/iamauth
// MintedIdentityHeaders), so a test that sets them is modelling the trusted edge,
// not a forgeable client.
func ctxFor(org, user, claim string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/billing/me/balance", nil)
	if org != "" {
		c.Set("organization", &organization.Organization{Name: org})
	}
	if user != "" {
		c.Request.Header.Set("X-User-Id", user)
	}
	if claim != "" {
		c.Request.Header.Set("X-Billing-Account-Id", claim)
	}
	return c
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
			got := payer(ctxFor(tc.org, tc.user, tc.claim)).Subject()
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

	if got := payer(ctxFor("hanzo", "alice", "")).Subject(); got != "hanzo/alice" {
		t.Fatalf("with hostile ORG_BILLING_ORGS: subject = %q, want %q (env must be inert)", got, "hanzo/alice")
	}
	if got := payer(ctxFor("acme", "bob", "")).Subject(); got != "acme" {
		t.Fatalf("with hostile PERSONAL_BILLING_ORGS: subject = %q, want %q (env must be inert)", got, "acme")
	}
}

// TestPayer_ForeignClaimIsRefused: a claim naming ANOTHER tenant's ledger is
// discarded, not billed. IAM never mints one, so this can only fire on a mis-wired
// caller — and it must degrade to the caller's own account, never redirect a debit
// into someone else's.
func TestPayer_ForeignClaimIsRefused(t *testing.T) {
	got := payer(ctxFor("acme", "bob", "org:victim")).Subject()
	if got == "victim" {
		t.Fatalf("foreign claim billed another tenant's ledger (%q) — cross-tenant debit", got)
	}
	if got != "acme" {
		t.Fatalf("foreign claim subject = %q, want fallback to own org %q", got, "acme")
	}
}

// TestPayer_FailsClosed: no org resolved means the caller cannot be attributed.
// It must resolve to no account — handlers 401 rather than bill someone free or,
// worse, bill a guess.
func TestPayer_FailsClosed(t *testing.T) {
	a := payer(ctxFor("", "alice", "org:acme")) // a claim cannot supply a missing org
	if !a.Zero() {
		t.Fatalf("payer with no org = %+v, want Zero", a)
	}
	if got := userBillingKey(ctxFor("", "alice", "")); got != "" {
		t.Fatalf("userBillingKey with no org = %q, want empty", got)
	}
}

// TestUserBillingKey_IsThePayer pins the one seam the balance/top-up handlers read
// through: the wallet a caller pays from IS the account the rule resolves. If these
// two ever diverge, a top-up and a read address different rows.
func TestUserBillingKey_IsThePayer(t *testing.T) {
	for _, tc := range []struct{ org, user, claim string }{
		{"hanzo", "alice", ""},
		{"acme", "bob", ""},
		{"acme", "bob", "person:acme/bob"},
	} {
		c := ctxFor(tc.org, tc.user, tc.claim)
		if got, want := userBillingKey(c), payer(c).Subject(); got != want {
			t.Fatalf("userBillingKey = %q, payer = %q — the read and the rule disagree", got, want)
		}
	}
}
