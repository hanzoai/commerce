package gateway

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/payment/providers/circle"
)

func holdItems(t *testing.T, rec *remittance.Remittance) map[string]int64 {
	t.Helper()
	db := systemDB()
	h := remittance.NewHold(db)
	if err := h.Get(db.NewKey(h.Kind(), holdID(rec), 0, nil)); err != nil && err != datastore.ErrNoSuchEntity {
		t.Fatal(err)
	}
	return h.Items
}

// RACE-A: credit() never checks the row's status. A credit() holding a row
// read while it was still Accepted (Submit's claim, or a second replica's
// advance) runs after the reconciler failed the payment: it reserves, marks
// Credited and LANDS on a Failed row, which nothing ever reverses, and its hold
// item is never released.
func TestRaceCreditLandsOnAFailedRow(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	ctx := context.Background()
	vA, err := r.submit(r.pay(MaxCents, nil)) // fills the hold
	if err != nil || !vA.Credited {
		t.Fatalf("A: %+v %v", vA, err)
	}
	vB, err := r.submit(r.pay(500, nil)) // deferred: hold full
	if err != nil || !vB.Deferred {
		t.Fatalf("B: %+v %v", vB, err)
	}
	stale := r.byID(vB.ID) // what Submit's claim read: Accepted, not credited

	r.gw.mark(vB.Payment, circle.Failed)
	r.pass()
	if got := r.byID(vB.ID); got.Status != remittance.Failed || got.Credited {
		t.Fatalf("setup: B = %+v", got)
	}
	r.gw.mark(vA.Payment, circle.Confirmed) // frees the hold
	r.pass()

	r.s.credit(ctx, stale) // the in-flight request finishes

	for i := 0; i < 3; i++ {
		r.pass()
	}
	got := r.byID(vB.ID)
	t.Logf("B after: status=%s credited=%v landed=%v reversed=%v; ledger=%d cents; hold=%v",
		got.Status, got.Credited, got.Landed, got.Reversed, r.credited(), holdItems(t, got))
	if got.Status == remittance.Failed && got.Landed && !got.Reversed {
		t.Errorf("EXPLOIT: a Failed payment's credit LANDED and is never reversed (ledger %d cents, want %d)", r.credited(), MaxCents)
	}
	if _, ok := holdItems(t, got)[vB.ID]; ok {
		t.Errorf("LEAK: a Failed row's hold item is held forever")
	}
}

// RACE-B: a row that reaches Settled with its credit owed but not landed (the
// ledger was down when Gateway answered completed) is never looked at again:
// Reconcile reads only accepted, pending and refused.
func TestRaceSettledWithTheCreditOwedIsNeverLanded(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	r.book.broken = true
	v, err := r.submit(r.pay(600, nil))
	if err != nil || v.Status != remittance.Accepted || v.Credited {
		t.Fatalf("submit: %+v %v", v, err)
	}
	r.gw.mark(v.Payment, circle.Completed)
	r.pass()
	r.book.mu.Lock()
	r.book.broken = false
	r.book.mu.Unlock()
	for i := 0; i < 5; i++ {
		r.pass()
	}
	// Presenting it again does not heal it either.
	_, _ = r.s.Get(context.Background(), r.org, r.payer, v.ID)
	got := r.byID(v.ID)
	t.Logf("after: status=%s credited=%v landed=%v ledger=%d", got.Status, got.Credited, got.Landed, r.credited())
	if got.Status == remittance.Settled && !got.Landed {
		t.Errorf("STUCK: settled, credit owed, never landed: the payer paid 600 cents and holds %d", r.credited())
	}
}

// RACE-B2: the same through the deferred path: a payment past the hold that
// Gateway reports completed while the ledger is down.
func TestRaceDeferredThenSettledWithLedgerDown(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	if _, err := r.submit(r.pay(MaxCents, nil)); err != nil {
		t.Fatal(err)
	}
	v, err := r.submit(r.pay(700, nil))
	if err != nil || !v.Deferred {
		t.Fatalf("submit: %+v %v", v, err)
	}
	r.book.mu.Lock()
	r.book.broken = true
	r.book.mu.Unlock()
	r.gw.mark(v.Payment, circle.Completed)
	r.pass()
	r.book.mu.Lock()
	r.book.broken = false
	r.book.mu.Unlock()
	for i := 0; i < 5; i++ {
		r.pass()
	}
	got := r.byID(v.ID)
	t.Logf("after: status=%s deferred=%v credited=%v landed=%v ledger=%d", got.Status, got.Deferred, got.Credited, got.Landed, r.credited())
	if got.Status == remittance.Settled && !got.Landed {
		t.Errorf("STUCK: deferred payment settled with credit owed and never landed")
	}
}

// A credit marked owed after fail's check must not be buried under a failed
// row: a failed row is never looked at again, so the money would stay credited
// with nothing to take it back. The last write refuses, the row stays open, and
// the next pass reverses the credit and fails the row.
func TestFailedNeverBuriesAnOwedCredit(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	v, err := r.submit(r.pay(600, nil))
	if err != nil || !v.Credited || !r.byID(v.ID).Landed {
		t.Fatalf("submit: %+v %v", v, err)
	}

	got, err := markFailed(v.ID, "failed")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status == remittance.Failed {
		t.Fatalf("a row holding a credit that was never taken back ended failed: %+v", got)
	}

	r.gw.mark(v.Payment, circle.Failed)
	for i := 0; i < 3; i++ {
		r.pass()
	}
	end := r.byID(v.ID)
	if end.Status != remittance.Failed || !end.Reversed || r.credited() != 0 {
		t.Fatalf("after the failure: status=%s reversed=%v ledger=%d cents, want failed, reversed, 0", end.Status, end.Reversed, r.credited())
	}
}
