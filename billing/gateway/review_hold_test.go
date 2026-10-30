package gateway

import (
	"context"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/payment/providers/circle"
)

// A stale credit() (read while the row was Accepted/received, not credited)
// finishing AFTER the reconciler moved the row to Settled and landed it:
// reserve() adds a hold item, the Credited swap is a no-op (already credited),
// and nothing ever releases the item: Settled+Landed rows are never swept.
func TestHoldLeaksOnSettledRow(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	ctx := context.Background()
	vA, err := r.submit(r.pay(MaxCents, nil))
	if err != nil || !vA.Credited {
		t.Fatalf("A: %+v %v", vA, err)
	}
	vB, err := r.submit(r.pay(500, nil))
	if err != nil || !vB.Deferred {
		t.Fatalf("B: %+v %v", vB, err)
	}
	stale := r.byID(vB.ID)
	r.gw.mark(vA.Payment, circle.Confirmed)
	r.gw.mark(vB.Payment, circle.Completed)
	r.pass()
	if got := r.byID(vB.ID); got.Status != remittance.Settled || !got.Landed {
		t.Fatalf("setup: %+v", got)
	}
	r.s.credit(ctx, stale)
	for i := 0; i < 5; i++ {
		r.pass()
	}
	items := holdItems(t, r.byID(vB.ID))
	t.Logf("hold after: %v ledger=%d", items, r.credited())
	if _, ok := items[vB.ID]; ok {
		t.Errorf("LEAK: settled row %s holds %d cents of the payer's unconfirmed cap forever", vB.ID, items[vB.ID])
	}
}

// Gateway failed an accepted, credited payment and the clawback keeps failing:
// the row must stay open, be retried with backoff (Due moves), and finish once
// the ledger answers.
func TestUnreversibleRowBacksOffThenFinishes(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	v, err := r.submit(r.pay(600, nil))
	if err != nil || !v.Credited {
		t.Fatalf("submit: %+v %v", v, err)
	}
	r.book.mu.Lock()
	r.book.broken = true
	r.book.mu.Unlock()
	r.gw.mark(v.Payment, circle.Failed)
	var dues []string
	for i := 0; i < 8; i++ {
		r.pass()
		got := r.byID(v.ID)
		if got.Status != remittance.Accepted || !got.Credited || got.Reversed {
			t.Fatalf("pass %d: %+v", i, got)
		}
		dues = append(dues, got.Due)
	}
	for i := 1; i < len(dues); i++ {
		if dues[i] <= dues[i-1] {
			t.Errorf("Due did not move: %v", dues)
		}
	}
	r.book.mu.Lock()
	r.book.broken = false
	r.book.mu.Unlock()
	for i := 0; i < 3; i++ {
		r.pass()
	}
	end := r.byID(v.ID)
	if end.Status != remittance.Failed || !end.Reversed || r.credited() != 0 {
		t.Fatalf("end: %+v ledger %d", end, r.credited())
	}
}

// An amount longer than 2^256 has digits is no amount: it fails without being
// parsed, and the same value written with leading zeroes still matches.
func TestAnAmountLongerThanAWordIsNotAnAmount(t *testing.T) {
	long := strings.Repeat("1", 79)
	if sameAmount(long, long) {
		t.Fatal("a 79-digit string matched itself")
	}
	if !sameAmount("00010000", "10000") {
		t.Fatal("leading zeroes no longer compare by value")
	}
}
