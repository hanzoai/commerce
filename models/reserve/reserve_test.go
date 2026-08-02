// Copyright © 2026 Hanzo AI. MIT License.

package reserve_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/reserve"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// reserve_test.go pins what this package DEMANDS OF THE STORE.
//
// The ceiling holds because the clamp is computed and written inside ONE
// indivisible transaction. That is a two-part contract: this package must ASK
// for indivisibility (pinned here, deterministically, on every run) and the
// store must DELIVER it (pinned in db/transaction_pg_test.go, deterministically,
// against real Postgres). The end-to-end contention test in reserve_pg_test.go
// is a regression over both — it is not the gate, because an interleaving test
// passes on a broken store whenever the scheduler is kind.

// witness is a store that records what a caller asked its transactions for, and
// otherwise IS the store it wraps.
type witness struct {
	db.DB
	asked []*db.TransactionOptions
}

func (w *witness) RunInTransaction(ctx context.Context, fn func(db.Transaction) error, opts *db.TransactionOptions) error {
	w.asked = append(w.asked, opts)
	return w.DB.RunInTransaction(ctx, fn, opts)
}

// declared is one reserve on a store that watches how it is written.
func declared(t *testing.T, ctx ae.Context, cap0 int64) (*datastore.Datastore, *control.Control, *witness) {
	t.Helper()
	w := &witness{DB: ctx.DB()}
	ds := datastore.NewWithDB(ctx, w)

	c := control.New(ds)
	c.Effect = control.Reserve
	c.SubjectKind = "merchant"
	c.Subject = "m1"
	c.Rate = control.FullRate
	c.Cap = cap0
	c.Currency = currency.USD
	if err := c.Create(); err != nil {
		t.Fatalf("declare: %v", err)
	}
	w.asked = nil // the declaration is not a movement
	return ds, c, w
}

func hold(screen string) reserve.Cause {
	return reserve.Cause{SubjectKind: "merchant", Subject: "m1", Currency: currency.USD, Screen: screen}
}

// TestTake_AsksTheStoreForIndivisibility — SB-3's first half. The ceiling is
// clamped against the total the store holds AT THAT INSTANT, which only means
// anything if the read and the write cannot be split by another disbursement.
// So every movement asks for serializable isolation and a budget to be re-run
// on refusal; nothing here may write outside that ask.
func TestTake_AsksTheStoreForIndivisibility(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	ds, c, w := declared(t, ctx, 1000)

	if _, err := reserve.Take(ds, c, 400, hold("s1")); err != nil {
		t.Fatalf("take: %v", err)
	}
	if _, err := reserve.Return(ds, c, "s1"); err != nil {
		t.Fatalf("return: %v", err)
	}
	if _, err := reserve.Close(ds, c); err != nil {
		t.Fatalf("close: %v", err)
	}

	if len(w.asked) != 3 {
		t.Fatalf("%d movements reached the store, want one per act (take, return, close)", len(w.asked))
	}
	for i, o := range w.asked {
		if o == nil {
			t.Fatalf("movement %d ran with no isolation stated — the default loses updates", i)
		}
		if o.Isolation != db.IsolationSerializable {
			t.Fatalf("movement %d asked for isolation %v, want serializable", i, o.Isolation)
		}
		if o.MaxAttempts < 2 {
			t.Fatalf("movement %d states a retry budget of %d — a refused transaction is "+
				"then a failed payout rather than a slower one", i, o.MaxAttempts)
		}
	}
}

// TestTake_RefusesWhenTheStoreCannotBeIndivisible — the account never falls
// back to two writes. Without one transaction the total and the ledger can
// half-land, and a disbursement that cannot be accounted for does not happen.
func TestTake_RefusesWhenTheStoreCannotBeIndivisible(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	_, c, _ := declared(t, ctx, 1000)

	for name, ds := range map[string]*datastore.Datastore{
		"no datastore": nil,
		"no store":     datastore.NewWithDB(context.Background(), nil),
	} {
		if _, err := reserve.Take(ds, c, 100, hold("s1")); !errors.Is(err, reserve.ErrStore) {
			t.Fatalf("%s: take answered %v, want ErrStore", name, err)
		}
		if _, err := reserve.Return(ds, c, "s1"); !errors.Is(err, reserve.ErrStore) {
			t.Fatalf("%s: return answered %v, want ErrStore", name, err)
		}
		if _, err := reserve.Close(ds, c); !errors.Is(err, reserve.ErrStore) {
			t.Fatalf("%s: close answered %v, want ErrStore", name, err)
		}
	}
}

// TestTake_RefusesAMovementItCannotName — a hold that names no judgement cannot
// be joined back to the decision that took it, and a movement of nothing is
// noise in an account a merchant has to read.
func TestTake_RefusesAMovementItCannotName(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	ds, c, _ := declared(t, ctx, 1000)

	if _, err := reserve.Take(ds, c, 100, reserve.Cause{}); !errors.Is(err, reserve.ErrCause) {
		t.Fatalf("a nameless hold was accepted: %v", err)
	}
	if _, err := reserve.Take(ds, c, 0, hold("s1")); !errors.Is(err, reserve.ErrAmount) {
		t.Fatalf("a zero movement was accepted: %v", err)
	}
	if _, err := reserve.Take(ds, nil, 100, hold("s1")); !errors.Is(err, reserve.ErrControl) {
		t.Fatalf("a movement against nothing was accepted: %v", err)
	}

	notAReserve := control.New(ds)
	notAReserve.Effect = control.Hold
	notAReserve.SubjectKind, notAReserve.Subject = "merchant", "m1"
	if err := notAReserve.Create(); err != nil {
		t.Fatalf("place a hold: %v", err)
	}
	if _, err := reserve.Take(ds, notAReserve, 100, hold("s1")); !errors.Is(err, reserve.ErrControl) {
		t.Fatalf("a hold withheld money: %v", err)
	}
}

// TestHeadroom_IsTheCeilingLessTheAccount — the forecast a judgement clamps
// against, and the only thing on this plane that reads the total without
// moving it.
func TestHeadroom_IsTheCeilingLessTheAccount(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	ds, c, _ := declared(t, ctx, 1000)

	if got := reserve.Headroom(ds, c); got != 1000 {
		t.Fatalf("headroom=%d on an untouched reserve, want its whole ceiling", got)
	}
	if _, err := reserve.Take(ds, c, 400, hold("s1")); err != nil {
		t.Fatalf("take: %v", err)
	}
	if got := reserve.Headroom(ds, c); got != 600 {
		t.Fatalf("headroom=%d after 400 was withheld, want 600", got)
	}

	// A reserve that declares no ceiling has room for anything, and a full one
	// has none — never a negative, which a clamp would read as a bound.
	open := control.New(ds)
	open.Effect, open.SubjectKind, open.Subject, open.Rate = control.Reserve, "merchant", "m2", control.FullRate
	if err := open.Create(); err != nil {
		t.Fatalf("place: %v", err)
	}
	if got := reserve.Headroom(ds, open); got != reserve.Unlimited {
		t.Fatalf("headroom=%d on a reserve with no ceiling", got)
	}
	if _, err := reserve.Take(ds, c, 5000, hold("s2")); err != nil {
		t.Fatalf("take: %v", err)
	}
	if got := reserve.Headroom(ds, c); got != 0 {
		t.Fatalf("headroom=%d at a full ceiling", got)
	}
	if got := reserve.Held(ds, c); got != 1000 {
		t.Fatalf("the account holds %d against a ceiling of 1000", got)
	}
}
