package store

import (
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

func TestHasAccessIsStoreBound(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	db := datastore.New(ctx)
	now := time.Now()

	sub := subscription.New(db)
	sub.UserId = "acme"
	sub.StoreId = "store-a"
	sub.PlanId = "pro"
	sub.Status = subscription.Active
	sub.PeriodStart = now.Add(-time.Hour)
	sub.PeriodEnd = now.Add(time.Hour)
	if err := sub.Create(); err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	allowed, status, err := HasAccess(db, "store-a", now)
	if err != nil || !allowed || status != "active" {
		t.Fatalf("store-a access = %v %q err=%v, want active", allowed, status, err)
	}
	allowed, status, err = HasAccess(db, "store-b", now)
	if err != nil || allowed || status != "payment_required" {
		t.Fatalf("store-b access = %v %q err=%v, want payment_required", allowed, status, err)
	}
}

func TestHasAccessRejectsExpiredTrial(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	db := datastore.New(ctx)
	now := time.Now()

	sub := subscription.New(db)
	sub.UserId = "acme"
	sub.StoreId = "store-a"
	sub.PlanId = "pro"
	sub.Status = subscription.Trialing
	sub.TrialStart = now.Add(-8 * 24 * time.Hour)
	sub.TrialEnd = now.Add(-24 * time.Hour)
	if err := sub.Create(); err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	allowed, status, err := HasAccess(db, "store-a", now)
	if err != nil || allowed || status != "payment_required" {
		t.Fatalf("expired trial access = %v %q err=%v, want payment_required", allowed, status, err)
	}
}
