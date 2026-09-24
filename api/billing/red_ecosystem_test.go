package billing

import (
	"io"

	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/models/transaction"
	txutil "github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A plain member names ANOTHER member of the same org as the subscription's
// subject and pays with "balance". The subject is taken from the request body and
// bounded only by the org prefix, and the balance debited is the subject's, so the
// other member's money buys a plan they did not ask for. Only the member
// themselves, an org admin, a SuperAdmin or a service should be able to name a
// subject other than the caller's own.
func TestRed_AMemberCannotSpendAnotherMembersBalance(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("acme-members")
	// DeductFromBalance reads kind "user" through the request context, so bob's
	// funds are seeded where that read looks.
	seedUserFunds(t, ctx, "acme-members/bob", 100000)
	before := userFunds(t, ctx, "acme-members/bob")

	alice := map[string]string{"X-User-IsOrgAdmin": "false", "X-User-Id": "acme-members/alice"}
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"balance","planId":"team","quantity":2,"userId":"acme-members/bob"}`, alice)
	body, _ := io.ReadAll(resp.Body)

	subs := parentSub(t, datastore.New(org.Namespaced(ctx)), "acme-members/bob", "team")
	after := userFunds(t, ctx, "acme-members/bob")
	t.Logf("bob's team subscription opened by alice: %v", subs != nil)
	t.Logf("status=%d balance %d -> %d body=%s", resp.StatusCode, before, after, body)
	if resp.StatusCode == http.StatusCreated || subs != nil {
		t.Fatalf("alice opened a subscription for bob paid from bob's balance: status=%d balance %d -> %d (%s)",
			resp.StatusCode, before, after, body)
	}
}

// The customer paths the branch closes, asked through every site that used the
// org-prefix rule: a person in each ecosystem org, including the signup org's
// customers, is not Enterprise, gets no floor and no ecosystem grant. Covered by
// the branch's own test for TierOf/ReadTier/EnsureEcosystemCredits; this adds the
// no-card subscribe path, where a member names themselves and sends no card.
func TestRed_ACustomerGetsNoFreeSubscription(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("hanzo")
	member := map[string]string{"X-User-IsOrgAdmin": "false", "X-User-Id": "hanzo/carol"}
	resp := invokeSubscribeCard(org, ctx, `{"planId":"team","quantity":2,"userId":"hanzo/carol"}`, member)
	cbody, _ := io.ReadAll(resp.Body)
	t.Logf("carol: status=%d body=%s", resp.StatusCode, cbody)
	if resp.StatusCode == http.StatusCreated {
		body := cbody
		t.Fatalf("a hanzo customer subscribed with no card and no balance: %s", body)
	}
	if got, _ := TierOf(ctx, org, "hanzo/carol"); got == tier.Enterprise {
		t.Fatal("a hanzo customer reads as Enterprise")
	}
	db := datastore.New(org.Namespaced(ctx))
	if grants, _ := getActiveGrants(db, "hanzo/carol"); len(grants) != 0 {
		t.Fatalf("a hanzo customer holds %d ecosystem grant(s)", len(grants))
	}
}

func seedUserFunds(t *testing.T, ctx ae.Context, subject string, cents int64) {
	t.Helper()
	tr := transaction.New(datastore.New(ctx))
	tr.Type = transaction.Deposit
	tr.DestinationId = subject
	tr.DestinationKind = "user"
	tr.Currency = currency.USD
	tr.Amount = currency.Cents(cents)
	tr.MustCreate()
}

func userFunds(t *testing.T, ctx ae.Context, subject string) currency.Cents {
	t.Helper()
	d, err := txutil.GetTransactionsByCurrency(ctx, subject, "user", currency.USD, false)
	if err != nil {
		t.Fatal(err)
	}
	if b, ok := d.Data[currency.USD]; ok && b != nil {
		return b.Balance
	}
	return 0
}

// A plain member of a pooled ecosystem org (lux, zoo) carries no billing claim and
// pays from the org's pool; that pool is the org's own account and is Enterprise.
// A signup-org customer pays from a person wallet that is not.
func TestRed_PooledMembersPayFromTheEcosystemAccount(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	for _, tc := range []struct {
		org, user, claim, subject string
		enterprise                bool
	}{
		{"lux", "dev", "", "lux", true},
		{"zoo", "dev", "", "zoo", true},
		{"lux", "lux/owner", "org:lux", "lux", true},
		{"hanzo", "hanzo/carol", "person:hanzo/carol", "hanzo/carol", false},
		{"hanzo", "carol", "", "hanzo/carol", false},
	} {
		got := subjectFor(t, tc.org, tc.user, tc.claim)
		if got != tc.subject {
			t.Errorf("%s in %s (claim %q) pays from %q, want %q", tc.user, tc.org, tc.claim, got, tc.subject)
			continue
		}
		name, err := TierOf(ctx, moneyOrg(tc.org), got)
		if err != nil {
			t.Fatal(err)
		}
		if (name == tier.Enterprise) != tc.enterprise {
			t.Errorf("%s pays from %s at tier %s, want enterprise=%v", tc.user, got, name, tc.enterprise)
		}
	}
}

// The same subject choice with sourceId "credits": the credit grants burned are
// the named subject's, in the org's own books, so a member spends another
// member's credit (a starter grant, a plan allotment) on a plan for them.
func TestRed_AMemberCannotBurnAnotherMembersCredits(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("acme-credits")
	db := datastore.New(org.Namespaced(ctx))
	g := creditgrant.New(db)
	g.UserId, g.Name = "acme-credits/bob", "starter"
	g.AmountCents, g.RemainingCents, g.Currency, g.Priority = 10000, 10000, currency.USD, 1
	if err := g.Create(); err != nil {
		t.Fatal(err)
	}

	alice := map[string]string{"X-User-IsOrgAdmin": "false", "X-User-Id": "acme-credits/alice"}
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"credits","planId":"team","quantity":2,"userId":"acme-credits/bob"}`, alice)
	body, _ := io.ReadAll(resp.Body)
	grants, _ := getActiveGrants(db, "acme-credits/bob")
	var left int64
	for _, x := range grants {
		left += x.RemainingCents
	}
	t.Logf("status=%d bob's credit 10000 -> %d body=%s", resp.StatusCode, left, body)
	if resp.StatusCode == http.StatusCreated || left != 10000 {
		t.Fatalf("alice spent bob's credit on a plan for bob: status=%d, bob's credit 10000 -> %d", resp.StatusCode, left)
	}
}
