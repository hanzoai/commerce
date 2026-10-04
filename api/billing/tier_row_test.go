package billing

import (
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// seedRow writes one live subscription on a catalog plan the way production opens
// one, its period in hand starting at start.
func seedRow(t *testing.T, db *datastore.Datastore, subject, slug, provider string, start time.Time) *subscription.Subscription {
	t.Helper()
	p, err := resolveSubscriptionPlan(db, slug)
	if err != nil {
		t.Fatalf("resolve plan %q: %v", slug, err)
	}
	s := subscription.New(db)
	s.UserId = subject
	s.ProviderType = provider
	s.Quantity = 1
	engine.StartSubscription(s, p)
	s.Status = subscription.Active
	s.TrialStart, s.TrialEnd = time.Time{}, time.Time{}
	s.PeriodStart, s.PeriodEnd = start, start.AddDate(0, 1, 0)
	if err := s.Create(); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return s
}

// The tier answer names the row its plan is served from, and a paid plan outranks a
// free row whose period began after it.
func TestReadTier_NamesTheRowThatServesThePlan(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("tier-row")
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	paid := seedRow(t, db, "tier-row/alice", "dev", "square", now.AddDate(0, 0, -20))
	seedRow(t, db, "tier-row/alice", "free", "internal", now.AddDate(0, 0, -5))

	name, err := TierOf(ctx, org, "tier-row/alice")
	if err != nil || name != tier.Pro {
		t.Fatalf("TierOf = %q, %v; want pro", name, err)
	}
	view, err := ReadTier(ctx, org, "tier-row/alice")
	if err != nil {
		t.Fatalf("ReadTier: %v", err)
	}
	if view.Plan != "dev" || view.Subscription != paid.Id() {
		t.Fatalf("plan %q from row %q; want dev from the paid row %q", view.Plan, view.Subscription, paid.Id())
	}

	if view, err := ReadTier(ctx, org, "tier-row/nobody"); err != nil || view.Subscription != "" {
		t.Fatalf("no subscription: row %q, %v; want none", view.Subscription, err)
	}
}

// GET /v1/billing/tier carries the serving row as `subscription`.
func TestGetTier_CarriesTheServingRow(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	org := moneyOrg("tier-row-wire")
	paid := seedRow(t, tierDeriveDB(org), "tier-row-wire/paige", "dev", "square", time.Now().AddDate(0, 0, -3))

	code, out := driveTierJSON(org, "/v1/billing/tier", "/v1/billing/tier?user=tier-row-wire/paige", GetTier)
	if code != 200 || out["subscription"] != paid.Id() || out["plan"] != "dev" {
		t.Fatalf("status %d subscription %v plan %v; want 200 from %s on dev", code, out["subscription"], out["plan"], paid.Id())
	}
}
