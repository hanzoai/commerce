// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package organization

import (
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/test/ae"
)

// born sets Born for one test and restores it after.
func born(t *testing.T, k Kind) {
	t.Helper()
	prev := Born
	Born = k
	t.Cleanup(func() { Born = prev })
}

// stored reads the org named name back from the store, fresh.
func stored(t *testing.T, db *datastore.Datastore, name string) *Organization {
	t.Helper()
	o := New(db)
	found, err := o.Query().Filter("Name=", name).Get()
	if err != nil || !found {
		t.Fatalf("read back %q: found=%v err=%v", name, found, err)
	}
	return o
}

// An undeclared deployment creates sandbox tenants: the fail-closed default.
func TestBornUndeclaredIsTest(t *testing.T) {
	if Born != Test {
		t.Fatalf("Born = %q with nothing declared, want %q", Born, Test)
	}
	ctx := ae.NewContext()
	defer ctx.Close()
	db := datastore.New(ctx)

	o := New(db)
	o.Name = "fresh-undeclared"
	if err := o.GetOrCreate("Name=", o.Name); err != nil {
		t.Fatal(err)
	}
	if got := stored(t, db, "fresh-undeclared"); got.Live {
		t.Fatal("an org created with nothing declared was born live")
	}
}

// A deployment that declares Live creates tenants that can pay, through the
// exact call the auth path makes.
func TestBornLiveCreatesLiveOrg(t *testing.T) {
	born(t, Live)
	ctx := ae.NewContext()
	defer ctx.Close()
	db := datastore.New(ctx)

	o := New(db)
	o.Name = "fresh-live"
	if err := o.GetOrCreate("Name=", o.Name); err != nil {
		t.Fatal(err)
	}
	got := stored(t, db, "fresh-live")
	if !got.Live || got.TestMode() || got.SquareEnvironment() != "production" {
		t.Fatalf("born live: Live=%v TestMode=%v env=%q, want a production org",
			got.Live, got.TestMode(), got.SquareEnvironment())
	}
}

// Born is read at creation only. An org that already exists keeps the kind its
// record states, so declaring Live never moves a stored test org — that stays
// the one explicit act it was.
func TestBornNeverMovesAnExistingOrg(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	db := datastore.New(ctx)

	o := New(db)
	o.Name = "existing-test"
	if err := o.GetOrCreate("Name=", o.Name); err != nil {
		t.Fatal(err)
	}

	born(t, Live)
	again := New(db)
	again.Name = "existing-test"
	if err := again.GetOrCreate("Name=", again.Name); err != nil {
		t.Fatal(err)
	}
	if again.Live {
		t.Fatal("declaring Live moved an existing test org on resolve")
	}
	if got := stored(t, db, "existing-test"); got.Live {
		t.Fatal("declaring Live rewrote an existing test org's record")
	}
}

// Anything that is not exactly Live is Test, here as everywhere a Kind is read.
func TestBornUnknownKindIsTest(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	for _, k := range []Kind{"", "production", "LIVE", "mainnet"} {
		born(t, k)
		o := New(datastore.New(ctx))
		o.Name = "x"
		if err := o.BeforeCreate(); err != nil {
			t.Fatal(err)
		}
		if o.Live {
			t.Fatalf("Born=%q created a live org", k)
		}
	}
}
