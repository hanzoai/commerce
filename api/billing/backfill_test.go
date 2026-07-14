package billing

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingaccount"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestBackfillIsIdempotent proves the boot backfill seeds an account + org
// binding per org and is a no-op on rerun — the property that makes it safe to
// run on every start (mirrors IAM's BackfillDefaultProjects).
func TestBackfillIsIdempotent(t *testing.T) {
	ctx := context.Context(ae.NewContext())
	rootDb := datastore.New(ctx)

	// Two orgs in the root registry (unique names so other tests' orgs can't
	// disturb the per-org assertions below).
	for _, name := range []string{"bf-alpha", "bf-beta"} {
		o := organization.New(rootDb)
		o.Name = name
		o.FullName = name + " Inc"
		if err := o.Create(); err != nil {
			t.Fatalf("create org %s: %v", name, err)
		}
	}

	// First backfill: creates accounts (the count may include other tests' orgs in
	// the shared store, so assert only that OUR orgs are backfilled).
	if _, err := BackfillBillingAccounts(ctx); err != nil {
		t.Fatalf("backfill run 1: %v", err)
	}

	for _, slug := range []string{"bf-alpha", "bf-beta"} {
		db := datastore.New(nsCtx(ctx, slug))
		acct, err := billingaccount.Get(db, slug)
		if err != nil {
			t.Fatalf("account %q not backfilled: %v", slug, err)
		}
		if acct.OwnerKind != billingaccount.HolderOrg || acct.OwnerId != slug {
			t.Fatalf("account %q owner = %s/%s; want org/%s", slug, acct.OwnerKind, acct.OwnerId, slug)
		}
		binds, err := billingaccount.ForHolder(db, billingaccount.HolderOrg, slug)
		if err != nil || len(binds) != 1 || binds[0].AccountId != slug {
			t.Fatalf("org binding for %q = %v (err %v); want one → %q", slug, binds, err, slug)
		}
	}

	// Rerun: nothing new is created (idempotent).
	created2, err := BackfillBillingAccounts(ctx)
	if err != nil {
		t.Fatalf("backfill run 2: %v", err)
	}
	if created2 != 0 {
		t.Fatalf("rerun created %d accounts; want 0 (idempotent)", created2)
	}
}

// TestBackfilledOrgResolvesToItself: after backfill, a pooled org's chain is
// exactly [org] — the degenerate default, no regression.
func TestBackfilledOrgResolvesToItself(t *testing.T) {
	t.Setenv("PERSONAL_BILLING_ORGS", "hanzo") // bf-solo is pooled
	t.Setenv("ORG_BILLING_ORGS", "")
	resetBillingOrgSetsForTest()

	ctx := context.Context(ae.NewContext())
	rootDb := datastore.New(ctx)
	o := organization.New(rootDb)
	o.Name = "bf-solo"
	if err := o.Create(); err != nil {
		t.Fatalf("create org: %v", err)
	}
	if _, err := BackfillBillingAccounts(ctx); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	chain, err := resolveBilling(nsCtx(ctx, "bf-solo"), Scope{Org: "bf-solo", User: "someuser"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(chain) != 1 || chain[0] != "bf-solo" {
		t.Fatalf("chain = %v; want [bf-solo]", chain)
	}
}
