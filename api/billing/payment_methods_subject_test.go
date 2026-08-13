package billing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// WHOSE CARD IS IT. saveCard stores a vaulted card under the PAYER — its own
// parameter is named `subject`, and it writes that value to both CustomerId and
// UserId. The payer is account.Payer, the one shared rule: a person pays from
// "<org>/<user>", not from the org slug.
//
// The read side did not follow. ListPaymentMethods filtered a non-privileged
// caller on orgBillingKey — the ORG SLUG — and callerMayReachBillingSubject
// compared against the same slug. So a card written under "<org>/alice" was looked
// up under "<org>" and was not there: alice's own cards vanished from her own
// account, and get/update/detach/set-default answered 404 on a card she owns.
// Topup, on the same records, already used the payer and charged that card fine.
// One record cannot have two owners.
//
// This is the same split-subject bug TestTopupSubject_FollowsSignedIdentity_NotQuery
// guards on the top-up door — money landing on "hanzo" one call and "hanzo/alice"
// the next. That door was migrated to userBillingKey; these were not.
//
// The existing tenant suite cannot see it: seedCard writes CustomerId = the org
// slug ("exactly what orgBillingKey resolves"), so both halves agree on a key the
// product never writes for a person. These tests seed the way saveCard really does.

// pmSubjectNS is this file's OWN namespace. The suite shares one datastore, so a
// test that seeds into a namespace another test reads makes both of them lie.
const pmSubjectNS = "pm-subject-org"

// pmSubjectClaim is the signed `billing_account` naming the person who pays. It is
// how a person resolves inside a DEDICATED org — the signup-org fallback would work
// too, but it would force this file into the shared "hanzo" namespace and back into
// the pollution above. The two paths reach the same rule; TestCallerMayReach…
// covers the fallback, which needs no datastore.
const pmSubjectClaim = "person:" + pmSubjectNS + "/alice"

// pmSubject is the payer both halves must agree on: what saveCard stamps, and what
// a read must filter for.
const pmSubject = pmSubjectNS + "/alice"

// seedOwnedCard writes a saved card into org `ns` owned by `subject` — the payer
// key saveCard stamps, which for a person is "<org>/<user>" and not the org slug.
func seedOwnedCard(t *testing.T, ns, subject string) string {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))
	pm := paymentmethod.New(db)
	pm.CustomerId = subject
	pm.UserId = subject
	pm.Type = "card"
	pm.ProviderRef = "ccof:subject-test"
	pm.Card = &paymentmethod.CardDetails{Brand: "visa", Last4: "4242", ExpMonth: 12, ExpYear: 2032}
	if err := pm.Create(); err != nil {
		t.Fatalf("seed card for %s in %s: %v", subject, ns, err)
	}
	return pm.Id()
}

// listAsPerson drives GET /v1/billing/methods as a signed-in PERSON: the org in
// locals (what the gateway resolves) plus the server-minted X-User-Id and
// billing-account claim that name the human. No permission bit — this is an
// ordinary member, the caller the subject rule is for. A privileged caller
// bypasses the guard entirely and would prove nothing.
func listAsPerson(t *testing.T, ns, user, claim string) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/methods", nil)
	req.Header.Set("X-User-Id", user)
	if claim != "" {
		req.Header.Set("X-Billing-Account-Id", claim)
	}
	resp := driveSeeded(pmSeed(ns, false), "/v1/billing/methods", req, ListPaymentMethods)
	raw, _ := io.ReadAll(resp.Body)
	var got []map[string]any
	_ = json.Unmarshal(raw, &got)
	ids := make([]string, 0, len(got))
	for _, m := range got {
		if id, ok := m["id"].(string); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// TestListPaymentMethods_OwnerSeesOwnCard — the card alice saved is the card alice
// lists. Read and write must resolve the SAME key; this is the read half of the
// invariant saveCard sets.
func TestListPaymentMethods_OwnerSeesOwnCard(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	id := seedOwnedCard(t, pmSubjectNS, pmSubject)

	ids := listAsPerson(t, pmSubjectNS, "alice", pmSubjectClaim)
	if len(ids) == 0 {
		t.Fatalf("alice listed her own saved card and got NOTHING — the read keyed the org slug "+
			"while saveCard stamped the payer %q; her card is invisible in her own account", pmSubject)
	}
	if len(ids) != 1 || ids[0] != id {
		t.Fatalf("listed %v, want exactly the owned card [%s]", ids, id)
	}
}

// TestListPaymentMethods_OtherMemberRefused — the fix must not widen the read to
// the org. Every self-serve signup lands in the SAME org, so "scope it to the org"
// would put one customer's card in another customer's list. Asserted against a
// hostile caller, because a well-formed one cannot fail this way.
func TestListPaymentMethods_OtherMemberRefused(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	seedOwnedCard(t, pmSubjectNS, pmSubject)

	ids := listAsPerson(t, pmSubjectNS, "mallory", "person:"+pmSubjectNS+"/mallory")
	if len(ids) != 0 {
		t.Fatalf("mallory listed alice's card (%v) — same org, different payer; the subject is the only "+
			"boundary between two self-serve customers", ids)
	}
}

// TestCallerMayReachBillingSubject_OwnerAndStranger — the guard behind
// SetDefaultPaymentMethod, DetachPaymentMethod, UpdatePaymentMethod and
// GetPaymentMethod. It decides whether a caller may act on a record owned by
// ownerIDs, so it must accept the record's real owner and refuse everyone else.
// Driven directly: the guard is the unit, and every handler that calls it inherits
// whatever it answers.
//
// This one uses the SIGNUP-ORG fallback ("hanzo" + a bare X-User-Id, no claim),
// which is how the live self-serve population resolves today — the case the sibling
// tests cannot use without sharing the suite's busiest namespace.
func TestCallerMayReachBillingSubject_OwnerAndStranger(t *testing.T) {
	owner := ctxWithIdentity("hanzo", "alice", "", "")
	if !callerMayReachBillingSubject(owner, "hanzo/alice") {
		t.Fatalf("alice was refused her OWN card — the guard compared the org slug against the payer key " +
			"saveCard stamped, so set-default and detach answer 404 on a card she owns")
	}

	stranger := ctxWithIdentity("hanzo", "mallory", "", "")
	if callerMayReachBillingSubject(stranger, "hanzo/alice") {
		t.Fatalf("mallory reached alice's card — inside one org the payer is the only boundary")
	}
}

// TestBillingSubject_OrgAccountsUnaffected — a dedicated org and an org-account
// claim both pay from the ORG, so their subject IS the org slug and nothing changes
// for them. Pinned so the fix is understood as narrowing to the payer, not as
// "person keys everywhere": moving an org-paying caller onto a person key would
// strand the cards it already has.
func TestBillingSubject_OrgAccountsUnaffected(t *testing.T) {
	for _, tc := range []struct{ name, org, user, account, want string }{
		{"dedicated org, no claim", "acme", "bob", "", "acme"},
		{"org-account claim in the signup org", "hanzo", "alice", "org:hanzo", "hanzo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := ctxWithIdentity(tc.org, tc.user, tc.account, "")
			if got := userBillingKey(c); got != tc.want {
				t.Fatalf("userBillingKey = %q, want %q", got, tc.want)
			}
			if !callerMayReachBillingSubject(c, tc.want) {
				t.Fatalf("an org-paying caller was refused its own org-keyed card (%q)", tc.want)
			}
			if !strings.EqualFold(orgBillingKey(c), tc.want) {
				t.Fatalf("orgBillingKey %q and userBillingKey %q disagree for an org-paying caller — "+
					"they must agree exactly where the payer IS the org", orgBillingKey(c), tc.want)
			}
		})
	}
}
