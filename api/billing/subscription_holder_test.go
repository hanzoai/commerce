package billing

import (
	"testing"

	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// In a shared org every customer's subscriptions live in one namespace, so the org is
// not the boundary between them: the wallet is. A caller that names another wallet's
// subscription finds nothing, cannot end it and cannot put it back, and the row is
// untouched. The org's own authority (AnyHolder) reaches every row.
//
// Mutation proof: read the row with loadSubscription instead of heldSubscription and
// the other wallet's cancel and reactivate go through.
func TestSubscription_AnotherWalletsRowIsNotFound(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-holder", 20000)
	got, err := RecordSubscription(ctx, org, balanceIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	id := got.Subscription.ID

	for _, holder := range []string{"acme/mallory", "", "other"} {
		if _, err := CancelSubscription(ctx, org, holder, id, false); !IsSubscriptionNotFound(err) {
			t.Errorf("holder %q cancelled %s held by %s: %v, want not found", holder, id, balSubject, err)
		}
		if _, err := ReactivateSubscription(ctx, org, holder, id); !IsSubscriptionNotFound(err) {
			t.Errorf("holder %q reactivated %s held by %s: %v, want not found", holder, id, balSubject, err)
		}
	}
	if r := reload(t, db, id); r.Status != subscription.Active || r.EndCancel {
		t.Fatalf("another wallet's attempts changed the row: %s, cancel at end %t", r.Status, r.EndCancel)
	}

	// The holder itself, in any case, and the org's own authority.
	if _, err := CancelSubscription(ctx, org, "ACME", id, true); err != nil {
		t.Fatalf("the holder could not cancel its own subscription: %v", err)
	}
	if _, err := ReactivateSubscription(ctx, org, AnyHolder, id); err != nil {
		t.Fatalf("the org's authority could not reactivate: %v", err)
	}
}

func TestHolds(t *testing.T) {
	for _, tc := range []struct {
		holder, owner string
		want          bool
	}{
		{AnyHolder, "hanzo/alice", true},
		{"hanzo/alice", "hanzo/alice", true},
		{" Hanzo/Alice ", "hanzo/alice", true},
		{"hanzo/mallory", "hanzo/alice", false},
		{"", "hanzo/alice", false},
		{"", "", false},
		{"hanzo", "hanzo/alice", false},
	} {
		if got := Holds(tc.holder, tc.owner); got != tc.want {
			t.Errorf("Holds(%q, %q) = %t, want %t", tc.holder, tc.owner, got, tc.want)
		}
	}
}
