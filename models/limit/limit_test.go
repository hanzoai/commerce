// Copyright © 2026 Hanzo AI. MIT License.

package limit

import (
	"testing"

	"github.com/hanzoai/commerce/util/test/ae"
)

// A row is found by the address Bind gives it and by its scope, restated in place,
// stamped when it was, and gone once deleted; a second scope's row on the same name
// is a different row.
func TestARowIsFoundByItsScopeAndName(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	db := AuthorityDB(ctx)

	for _, r := range []struct {
		scope string
		value int64
	}{{"org:acme", 500}, {"plan:enterprise", 300}} {
		m := New(db)
		m.Name, m.Scope, m.Value, m.Reason, m.UpdatedBy = "sandbox.running", r.scope, r.value, "more sandboxes", "z@hanzo.ai"
		m.Bind()
		if err := m.Create(); err != nil {
			t.Fatalf("create %s: %v", r.scope, err)
		}
	}

	got := New(db)
	ok, err := got.Query().Filter("Slug=", "org:acme/sandbox.running").Get()
	if err != nil || !ok {
		t.Fatalf("by slug: found %v (%v)", ok, err)
	}
	if got.Value != 500 || got.Scope != "org:acme" || got.Name != "sandbox.running" || got.UpdatedAt.IsZero() {
		t.Fatalf("read %+v", got)
	}

	var plans []*Limit
	if _, err := Query(db).Filter("Scope=", "plan:enterprise").GetAll(&plans); err != nil || len(plans) != 1 || plans[0].Value != 300 {
		t.Fatalf("by scope: %d rows (%v)", len(plans), err)
	}

	got.Value, got.Reason = 800, "agency grew"
	if err := got.Update(); err != nil {
		t.Fatal(err)
	}
	again := New(db)
	if ok, err := again.Query().Filter("Slug=", "org:acme/sandbox.running").Get(); err != nil || !ok || again.Value != 800 || again.Reason != "agency grew" {
		t.Fatalf("restated: found %v value %d reason %q (%v)", ok, again.Value, again.Reason, err)
	}

	if err := again.Delete(); err != nil {
		t.Fatal(err)
	}
	gone := New(db)
	if ok, err := gone.Query().Filter("Slug=", "org:acme/sandbox.running").Get(); err != nil || ok {
		t.Fatalf("deleted row still found: %v (%v)", ok, err)
	}
	var all []*Limit
	if _, err := Query(db).GetAll(&all); err != nil || len(all) != 1 || all[0].Scope != "plan:enterprise" {
		t.Fatalf("after the delete %d rows (%v); want the plan's alone", len(all), err)
	}
}
