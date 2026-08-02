// Copyright © 2026 Hanzo AI. MIT License.

package control

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

func nsDB(parent context.Context, ns string) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(parent, ns))
}

func place(t *testing.T, db *datastore.Datastore, effect string, rate int64) *Control {
	t.Helper()
	c := New(db)
	c.Effect = effect
	c.SubjectKind = "merchant"
	c.Subject = "m1"
	c.Rate = rate
	if err := c.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}
	return c
}

// TestAQueryRowCannotBeWrittenThrough pins the ORM behaviour every write in
// this package is shaped around, so nobody has to rediscover it the way it was
// discovered: by watching a running total stay at zero while Update returned
// nil.
//
// A row handed back by a query iterator has no writable identity here. Update
// lands nowhere AND REPORTS NO ERROR. This test asserts the hazard is still
// real; if it ever starts failing, the ORM grew the ability and the reload in
// [Lift] can go — but until then, deleting the reload silently stops releases
// from landing.
func TestAQueryRowCannotBeWrittenThrough(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "iterwrite")

	placed := place(t, db, Reserve, 2500)

	live, err := LiveFor(db, "merchant", "m1", time.Now())
	if err != nil || len(live) != 1 {
		t.Fatalf("live=%d err=%v", len(live), err)
	}
	fromQuery := live[0]
	fromQuery.Reason = "written through a query row"
	if err := fromQuery.Update(); err != nil {
		t.Fatalf("update reported an error: %v", err)
	}

	back := New(db)
	if err := back.GetById(placed.Id()); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if back.Reason == "written through a query row" {
		t.Skip("the ORM now writes through query rows; the reload in Lift is no longer load-bearing")
	}
	if back.Reason != "" {
		t.Fatalf("reason=%q, want empty — this test exists to pin that a query row's write does NOT land", back.Reason)
	}
}

// TestNoBalanceLivesOnADeclaration is the structural half of the fix that moved
// the reserve's running total out of this package.
//
// A total on the control was a total every op that could read or place a
// declaration could move — which is how a money-free screen came to disarm a
// merchant's own ceiling. The balance now lives in models/reserve behind the one
// door that moves money, and this test pins that no field here can be mistaken
// for it again.
func TestNoBalanceLivesOnADeclaration(t *testing.T) {
	forbidden := map[string]bool{"Held": true, "Balance": true, "Withheld": true, "Total": true}
	typ := reflect.TypeOf(Control{})
	for i := 0; i < typ.NumField(); i++ {
		if forbidden[typ.Field(i).Name] {
			t.Fatalf("Control.%s is a balance on a declaration — it belongs to the account, "+
				"behind reserve.Take", typ.Field(i).Name)
		}
	}
}

// TestBounded_IsAboutTheDeclarationAndNothingElse — a ceiling is declared here;
// what has been withheld against it is the account's business.
func TestBounded_IsAboutTheDeclarationAndNothingElse(t *testing.T) {
	if !(&Control{Effect: Reserve, Rate: 2500, Cap: 1000, Currency: currency.USD}).Bounded() {
		t.Fatal("a reserve with a cap did not report itself bounded")
	}
	if (&Control{Effect: Reserve, Rate: 2500}).Bounded() {
		t.Fatal("a reserve with no cap reported itself bounded")
	}
	if (&Control{Effect: Block, Cap: 10}).Bounded() {
		t.Fatal("a block reported itself a bounded reserve")
	}
}

// TestBears_ScopesOnlyAReserveByCurrency — a hold and a block are about the
// subject, not about an amount, so they never narrow by currency.
func TestBears_ScopesOnlyAReserveByCurrency(t *testing.T) {
	usd := &Control{Effect: Reserve, Rate: 2500, Currency: currency.USD}
	if !usd.Bears(currency.USD) || usd.Bears(currency.EUR) {
		t.Fatalf("a USD reserve bears usd=%v eur=%v", usd.Bears(currency.USD), usd.Bears(currency.EUR))
	}
	any := &Control{Effect: Reserve, Rate: 2500}
	if !any.Bears(currency.USD) || !any.Bears(currency.EUR) {
		t.Fatal("a reserve naming no currency must bear on every currency")
	}
	for _, effect := range []string{Hold, Block} {
		c := &Control{Effect: effect, Currency: currency.USD}
		if !c.Bears(currency.EUR) {
			t.Fatalf("a %s stopped bearing on a move in another currency", effect)
		}
	}
}

// TestReadsAreBounded — no read in this package materialises a whole table, and
// no caller can ask one to.
func TestReadsAreBounded(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "bounded")

	for i := 0; i < Max+25; i++ {
		place(t, db, Hold, 0)
	}
	for _, limit := range []int{0, -1, Max * 10} {
		if n := len(All(db, limit)); n > Max {
			t.Fatalf("All(limit=%d) returned %d rows, want at most %d", limit, n, Max)
		}
	}
	live, err := LiveFor(db, "merchant", "m1", time.Now())
	if err != nil {
		t.Fatalf("livefor: %v", err)
	}
	if len(live) > Max {
		t.Fatalf("LiveFor returned %d rows, want at most %d", len(live), Max)
	}
	if n := len(All(db, 3)); n != 3 {
		t.Fatalf("All(limit=3) returned %d rows", n)
	}
}

// TestReadsAreTenantScoped — the datastore's namespace IS the boundary, and
// neither read takes an org it could widen.
func TestReadsAreTenantScoped(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	a, b := nsDB(c, "ctlisoa"), nsDB(c, "ctlisob")

	placed := place(t, a, Block, 0)
	if n := len(All(b, 0)); n != 0 {
		t.Fatalf("org B listed %d of org A's controls", n)
	}
	live, err := LiveFor(b, "merchant", "m1", time.Now())
	if err != nil {
		t.Fatalf("livefor: %v", err)
	}
	if len(live) != 0 {
		t.Fatalf("org B read %d of org A's live controls", len(live))
	}
	if _, err := Lift(b, placed.Id(), "b", time.Now()); err == nil {
		t.Fatal("org B lifted org A's control")
	}
}
