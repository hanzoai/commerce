package store

import (
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// storeSub writes a subscription bound to storeID on slug at cents, managed by
// provider, in status, current at now.
func storeSub(t *testing.T, db *datastore.Datastore, storeID, slug string, cents int64, provider string, status subscription.Status, now time.Time) *subscription.Subscription {
	t.Helper()
	sub := subscription.New(db)
	sub.UserId = "acme"
	sub.StoreId = storeID
	sub.PlanId = slug
	sub.Plan.Slug, sub.Plan.Category, sub.Plan.Price = slug, "personal", currency.Cents(cents)
	sub.ProviderType = provider
	sub.Status = status
	sub.PeriodStart = now.Add(-time.Hour)
	sub.PeriodEnd = now.Add(time.Hour)
	if err := sub.Create(); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	return sub
}

func TestHasAccessIsStoreBound(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	db := datastore.New(ctx)
	now := time.Now()
	storeSub(t, db, "store-a", "go", 800, "square", subscription.Active, now)

	allowed, status, err := HasAccess([]*datastore.Datastore{db}, "store-a", now)
	if err != nil || !allowed || status != "active" {
		t.Fatalf("store-a access = %v %q err=%v, want active", allowed, status, err)
	}
	allowed, status, err = HasAccess([]*datastore.Datastore{db}, "store-b", now)
	if err != nil || allowed || status != "payment_required" {
		t.Fatalf("store-b access = %v %q err=%v, want payment_required", allowed, status, err)
	}
}

// Any paid plan opens a store — one on sale, or a retired one its holder still
// pays for — and nothing else does: not a free plan, and not a row on a paid
// plan that no payment backs.
func TestHasAccessOpensOnAnyPaidPlan(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name, slug, provider string
		cents                int64
		want                 bool
	}{
		{"a rung on sale", "max-15x", "square", 9500, true},
		{"a retired rung still held", "dev", "square", 2000, true},
		{"the free plan", "free", "square", 0, false},
		{"a paid rung no payment backs", "go", "internal", 800, false},
	} {
		ctx := ae.NewContext()
		db := datastore.New(ctx)
		storeSub(t, db, "store-a", tc.slug, tc.cents, tc.provider, subscription.Active, now)
		allowed, _, err := HasAccess([]*datastore.Datastore{db}, "store-a", now)
		if err != nil || allowed != tc.want {
			t.Errorf("%s: access = %v err=%v, want %v", tc.name, allowed, err, tc.want)
		}
		ctx.Close()
	}
}

// A trial opened before commerce stopped selling them runs to its end and opens
// the store until then, and not after.
func TestHasAccessRunsAnOpenTrialToItsEnd(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	db := datastore.New(ctx)
	now := time.Now()
	for store, end := range map[string]time.Time{"store-open": now.Add(24 * time.Hour), "store-over": now.Add(-24 * time.Hour)} {
		sub := subscription.New(db)
		sub.UserId, sub.StoreId, sub.PlanId, sub.Status = "acme", store, "dev", subscription.Trialing
		sub.TrialStart, sub.TrialEnd = end.Add(-7*24*time.Hour), end
		if err := sub.Create(); err != nil {
			t.Fatalf("create subscription: %v", err)
		}
	}
	if allowed, status, err := HasAccess([]*datastore.Datastore{db}, "store-open", now); err != nil || !allowed || status != "trial" {
		t.Fatalf("an open trial: %v %q err=%v, want allowed as trial", allowed, status, err)
	}
	if allowed, status, err := HasAccess([]*datastore.Datastore{db}, "store-over", now); err != nil || allowed || status != "payment_required" {
		t.Fatalf("an expired trial: %v %q err=%v, want payment_required", allowed, status, err)
	}
}
