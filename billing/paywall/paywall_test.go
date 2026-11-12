package paywall

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	billing "github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/billing/invite"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

const org = "acme"

// newDB opens a store with the catalog seeded into the plan authority, as a boot
// does: the admin opens only on a Hanzo plan, a row of the authority.
func newDB(t *testing.T) (*datastore.Datastore, func()) {
	t.Helper()
	ctx := ae.NewContext()
	if _, _, err := billing.SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return datastore.New(ctx), func() { ctx.Close() }
}

// allow drives the decision over ONE store (subscriptions, trial credit and
// invites share the test db), mirroring how api/store's access_test creates and
// reads through the same handle.
func allow(db *datastore.Datastore, at time.Time) (bool, string, error) {
	return Allowed([]*datastore.Datastore{db}, db, db, org, at)
}

func seedSubscription(t *testing.T, db *datastore.Datastore, status subscription.Status, at time.Time) {
	t.Helper()
	sub := subscription.New(db)
	sub.UserId = org
	sub.PlanId = "go"
	sub.Plan.Slug, sub.Plan.Category, sub.Plan.Price = "go", "personal", 800
	sub.ProviderType = "square"
	sub.Status = status
	switch status {
	case subscription.Active:
		sub.PeriodStart = at.Add(-time.Hour)
		sub.PeriodEnd = at.Add(time.Hour)
	case subscription.Trialing:
		sub.TrialStart = at.Add(-24 * time.Hour)
		sub.TrialEnd = at.Add(24 * time.Hour)
	}
	if err := sub.Create(); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
}

// TestAllowed_Subscribed proves an org with a current active paid subscription is
// allowed (reason "active").
func TestAllowed_Subscribed(t *testing.T) {
	db, done := newDB(t)
	defer done()
	now := time.Now()

	seedSubscription(t, db, subscription.Active, now)

	ok, reason, err := allow(db, now)
	if err != nil || !ok || reason != reasonActive {
		t.Fatalf("subscribed access = %v %q err=%v, want allow/active", ok, reason, err)
	}
}

// TestAllowed_Trialing proves a trial opened before commerce stopped selling them
// (its window still open) is allowed (reason "trialing") until its end.
func TestAllowed_Trialing(t *testing.T) {
	db, done := newDB(t)
	defer done()
	now := time.Now()

	seedSubscription(t, db, subscription.Trialing, now)

	ok, reason, err := allow(db, now)
	if err != nil || !ok || reason != reasonTrialing {
		t.Fatalf("trialing access = %v %q err=%v, want allow/trialing", ok, reason, err)
	}
}

// trialCredit writes a store trial's credit as billing/trial wrote it, expiring at
// expires (zero: no expiry).
func trialCredit(t *testing.T, db *datastore.Datastore, expires time.Time) {
	t.Helper()
	tx := transaction.New(db)
	tx.Type = transaction.Deposit
	tx.DestinationId = org + "/alice"
	tx.DestinationKind = "iam-user"
	tx.Currency = "usd"
	tx.Amount = currency.Cents(2000)
	tx.Tags = "trial-credit"
	tx.ExpiresAt = expires
	if err := tx.Create(); err != nil {
		t.Fatalf("create trial credit: %v", err)
	}
}

// A trial opened before commerce stopped selling them runs to its end: its credit
// opens the admin until it expires, and not after; a credit that names no end
// opens nothing.
func TestAllowed_AnOpenTrialCreditRunsToItsEnd(t *testing.T) {
	now := time.Now()
	for name, c := range map[string]struct {
		expires time.Time
		allowed bool
	}{
		"open":      {now.Add(7 * 24 * time.Hour), true},
		"expired":   {now.Add(-24 * time.Hour), false},
		"no expiry": {time.Time{}, false},
	} {
		db, done := newDB(t)
		trialCredit(t, db, c.expires)
		ok, reason, err := allow(db, now)
		switch {
		case err != nil:
			t.Errorf("%s: %v", name, err)
		case c.allowed && (!ok || reason != reasonTrialCredit):
			t.Errorf("%s trial credit: %v %q, want allowed by its trial credit", name, ok, reason)
		case !c.allowed && (ok || reason != DeniedCode):
			t.Errorf("%s trial credit: %v %q, want deny/subscription_required", name, ok, reason)
		}
		done()
	}
}

// Any paid plan opens the admin — a rung on sale or a retired one still held —
// and a free plan, or a paid row no payment backs, does not.
func TestAllowed_AnyPaidPlan(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		slug, provider string
		cents          int64
		want           bool
	}{
		{"ultra", "square", 19500, true},
		{"max-20x", "square", 20000, true},
		{"free", "square", 0, false},
		{"pro-3x", "internal", 1500, false},
		{"merchant-box", "square", 2900, false},
	} {
		db, done := newDB(t)
		sub := subscription.New(db)
		sub.UserId, sub.PlanId, sub.ProviderType, sub.Status = org, tc.slug, tc.provider, subscription.Active
		sub.Plan.Slug, sub.Plan.Category, sub.Plan.Price = tc.slug, "personal", currency.Cents(tc.cents)
		sub.PeriodStart, sub.PeriodEnd = now.Add(-time.Hour), now.Add(time.Hour)
		if err := sub.Create(); err != nil {
			t.Fatalf("create %s: %v", tc.slug, err)
		}
		if ok, _, err := allow(db, now); err != nil || ok != tc.want {
			t.Errorf("%s by %s: allowed=%v err=%v, want %v", tc.slug, tc.provider, ok, err, tc.want)
		}
		done()
	}
}

// TestAllowed_InviteRedeemed proves an org that has redeemed an invite code —
// with no subscription — is allowed (reason "invite").
func TestAllowed_InviteRedeemed(t *testing.T) {
	db, done := newDB(t)
	defer done()
	now := time.Now()

	if _, err := invite.Mint(db, "WELCOME", "beta"); err != nil {
		t.Fatalf("mint invite: %v", err)
	}
	if _, redeemed, err := invite.Redeem(db, "WELCOME", org); err != nil || !redeemed {
		t.Fatalf("redeem invite = redeemed:%v err=%v, want redeemed", redeemed, err)
	}

	ok, reason, err := allow(db, now)
	if err != nil || !ok || reason != reasonInvite {
		t.Fatalf("invite access = %v %q err=%v, want allow/invite", ok, reason, err)
	}
}

// TestAllowed_None proves an org with neither access path is denied
// with the subscription_required code.
func TestAllowed_None(t *testing.T) {
	db, done := newDB(t)
	defer done()

	ok, reason, err := allow(db, time.Now())
	if err != nil || ok || reason != DeniedCode {
		t.Fatalf("no-access = %v %q err=%v, want deny/subscription_required", ok, reason, err)
	}
}

// TestAllowed_InviteForAnotherOrgRejected proves an invite redeemed by a
// DIFFERENT org does not unlock this org.
func TestAllowed_InviteForAnotherOrgRejected(t *testing.T) {
	db, done := newDB(t)
	defer done()

	if _, err := invite.Mint(db, "OTHER", ""); err != nil {
		t.Fatalf("mint invite: %v", err)
	}
	if _, _, err := invite.Redeem(db, "OTHER", "someone-else"); err != nil {
		t.Fatalf("redeem invite: %v", err)
	}

	ok, reason, err := allow(db, time.Now())
	if err != nil || ok || reason != DeniedCode {
		t.Fatalf("other-org invite = %v %q err=%v, want deny/subscription_required", ok, reason, err)
	}
}

// TestRequire_Denies402Shape drives the Require middleware end to end for an org
// with no access and asserts the 402 status and the {code:"subscription_required"}
// body contract the console reads.
func TestRequire_Denies402Shape(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	o := organization.New(datastore.New(ctx))
	o.Name = org

	reached := false
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(zip.H(func(c *zip.Ctx) error {
		c.Locals("organization", o)
		return c.Next()
	}))
	app.Use(zip.H(Require))
	app.Raw(http.MethodGet, "/x", func(c *zip.Ctx) error { reached = true; return c.NoContent(http.StatusOK) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusPaymentRequired || reached {
		t.Fatalf("no-access gate: status=%d reached=%v, want 402 & not-reached", resp.StatusCode, reached)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != DeniedCode {
		t.Fatalf("deny code = %q, want %q", body.Code, DeniedCode)
	}
}
