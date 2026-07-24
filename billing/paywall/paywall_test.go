package paywall

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/invite"
	"github.com/hanzoai/commerce/billing/trial"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

const org = "acme"

func newDB(t *testing.T) (*datastore.Datastore, func()) {
	t.Helper()
	ctx := ae.NewContext()
	return datastore.New(ctx), func() { ctx.Close() }
}

// allow drives the decision over ONE store (subscriptions, trial credit, and
// invites all share the test db), mirroring how api/store's access_test creates
// and reads through the same handle.
func allow(db *datastore.Datastore, at time.Time) (bool, string, error) {
	return Allowed([]*datastore.Datastore{db}, db, db, org, at)
}

func seedSubscription(t *testing.T, db *datastore.Datastore, status subscription.Status, at time.Time) {
	t.Helper()
	sub := subscription.New(db)
	sub.UserId = org
	sub.PlanId = trial.PlanSlug
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

// TestAllowed_Subscribed proves an org with a current active pro subscription is
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

// TestAllowed_Trialing proves an org with a trialing pro subscription (trial
// window still open) is allowed (reason "trialing").
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

// TestAllowed_TrialCredit proves an org with a live (unexpired, positive)
// trial-credit deposit — but no subscription — is allowed (reason "trial_credit").
func TestAllowed_TrialCredit(t *testing.T) {
	db, done := newDB(t)
	defer done()
	now := time.Now()

	tx := transaction.New(db)
	tx.Type = transaction.Deposit
	tx.DestinationId = org + "/alice"
	tx.DestinationKind = trial.Kind
	tx.Currency = "usd"
	tx.Amount = currency.Cents(2000)
	tx.Tags = trial.CreditTag
	tx.ExpiresAt = now.Add(7 * 24 * time.Hour)
	if err := tx.Create(); err != nil {
		t.Fatalf("create trial credit: %v", err)
	}

	ok, reason, err := allow(db, now)
	if err != nil || !ok || reason != reasonTrialCredit {
		t.Fatalf("trial-credit access = %v %q err=%v, want allow/trial_credit", ok, reason, err)
	}
}

// TestAllowed_TrialCreditExpiredRejected proves an EXPIRED trial credit does not
// unlock (falls through to deny).
func TestAllowed_TrialCreditExpiredRejected(t *testing.T) {
	db, done := newDB(t)
	defer done()
	now := time.Now()

	tx := transaction.New(db)
	tx.Type = transaction.Deposit
	tx.DestinationId = org + "/alice"
	tx.DestinationKind = trial.Kind
	tx.Currency = "usd"
	tx.Amount = currency.Cents(2000)
	tx.Tags = trial.CreditTag
	tx.ExpiresAt = now.Add(-24 * time.Hour) // expired
	if err := tx.Create(); err != nil {
		t.Fatalf("create expired trial credit: %v", err)
	}

	ok, reason, err := allow(db, now)
	if err != nil || ok || reason != DeniedCode {
		t.Fatalf("expired trial-credit access = %v %q err=%v, want deny/subscription_required", ok, reason, err)
	}
}

// TestAllowed_InviteRedeemed proves an org that has redeemed an invite code —
// with no subscription and no trial credit — is allowed (reason "invite").
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

// TestAllowed_None proves an org with none of the three access paths is denied
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
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	ctx := ae.NewContext()
	defer ctx.Close()

	o := organization.New(datastore.New(ctx))
	o.Name = org

	reached := false
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(func(c *zip.Ctx) error {
		c.Locals("organization", o)
		return c.Next()
	})
	app.Use(Require)
	app.Get("/x", func(c *zip.Ctx) error { reached = true; return c.NoContent(http.StatusOK) })

	resp, err := app.Fiber().Test(httptest.NewRequest(http.MethodGet, "/x", nil))
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
