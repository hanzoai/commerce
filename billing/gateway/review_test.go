// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/payment/providers/circle"
)

// Red's review of e3946bf06, each finding pinned as the behaviour it now has.

// R1 / M3: credit Gateway has only LOCKED is capped per payer at MaxCents in
// total. Past that, a payment is credited when Gateway confirms it.
func TestUnconfirmedCreditIsCappedPerPayer(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	r.gw.fund(r.from, 5*MaxCents)
	var views []View
	for i := 0; i < 5; i++ {
		v, err := r.submit(r.pay(MaxCents, nil))
		if err != nil || v.Status != remittance.Accepted {
			t.Fatalf("submit %d: %+v %v", i, v, err)
		}
		views = append(views, v)
	}
	if !views[0].Credited || views[0].Deferred {
		t.Fatalf("the first payment fits the hold and is credited on acceptance: %+v", views[0])
	}
	for _, v := range views[1:] {
		if v.Credited || !v.Deferred {
			t.Fatalf("a payment past the hold must wait for confirmation: %+v", v)
		}
	}
	if got := r.credited(); got != MaxCents {
		t.Fatalf("unconfirmed credit = %d cents, want at most %d", got, MaxCents)
	}

	// Payment 3 is confirmed onchain: credited, outside the hold.
	r.gw.mark(views[2].Payment, circle.Confirmed)
	r.pass()
	if got := r.credited(); got != 2*MaxCents {
		t.Fatalf("after one confirmation the ledger holds %d, want %d", got, 2*MaxCents)
	}
	// Payment 1 is confirmed: its room in the hold frees, and the next pass
	// credits one more on its lock.
	r.gw.mark(views[0].Payment, circle.Confirmed)
	r.pass()
	r.pass()
	if got := r.credited(); got != 3*MaxCents {
		t.Fatalf("after the hold freed the ledger holds %d, want %d", got, 3*MaxCents)
	}
	unconfirmed := 0
	for _, v := range views {
		rec := r.byID(v.ID)
		if rec.Landed && rec.State != circle.Confirmed {
			unconfirmed++
		}
	}
	if unconfirmed != 1 {
		t.Fatalf("%d unconfirmed payments are credited, want exactly 1", unconfirmed)
	}
}

// Two payments accepted at once for one payer cannot both take the last room.
func TestTheHoldIsExactUnderConcurrency(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	r.gw.fund(r.from, 10*MaxCents)
	subs := make([]Submission, 8)
	for i := range subs {
		subs[i] = r.pay(MaxCents/2, nil)
	}
	var wg sync.WaitGroup
	for _, sub := range subs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := r.submit(sub); err != nil {
				t.Errorf("submit: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := r.credited(); got != MaxCents {
		t.Fatalf("8 concurrent half-cap payments credited %d cents on acceptance, want exactly %d", got, MaxCents)
	}
}

// R2 / M1: Gateway holds ANOTHER payment under the nonce. This one can never
// settle: it fails for good at once and is never sent again.
func TestANonceSpentOnAnotherPaymentFailsForGood(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	sub := r.pay(100, nil)
	from, nonce := strings.ToLower(sub.Authorization.From), strings.ToLower(sub.Authorization.Nonce)
	r.gw.set(func(f *fake) {
		id := "11111111-2222-3333-4444-555555555555"
		f.payments[id] = &circle.Payment{ID: id, Status: circle.Completed, SendingNetwork: caip, RecipientNetwork: caip,
			From: from, To: "0x000000000000000000000000000000000000beef", Amount: "1", Nonce: nonce}
		f.byNonce[caip+"|"+from+"|"+nonce] = id
	})
	_, err := r.submit(sub)
	if ReasonOf(err) != ReasonConflict || StatusOf(err) != 409 {
		t.Fatalf("submit: %v", err)
	}
	if rec := r.stored(sub); rec.Status != remittance.Failed || rec.Reason != ReasonConflict || rec.Due != never {
		t.Fatalf("record = %+v", rec)
	}
	r.gw.mu.Lock()
	s0, f0 := r.gw.settles, r.gw.finds
	r.gw.mu.Unlock()
	for i := 0; i < 5; i++ {
		r.pass()
	}
	r.gw.mu.Lock()
	s1, f1 := r.gw.settles, r.gw.finds
	r.gw.mu.Unlock()
	if s1 != s0 || f1 != f0 {
		t.Fatalf("a failed row was sent to Gateway again: %d settles, %d finds", s1-s0, f1-f0)
	}
}

// R3 / L1: the credit is OWED before the ledger is asked. A credit that landed
// with its Landed flag lost is still clawed back when Gateway fails it.
func TestACreditWhoseFlagWasLostIsStillClawedBack(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	v, err := r.submit(r.pay(MaxCents, nil))
	if err != nil || !v.Credited {
		t.Fatalf("submit: %+v %v", v, err)
	}
	if _, err := update(v.ID, func(x *remittance.Remittance) bool { x.Landed = false; return true }); err != nil {
		t.Fatal(err)
	}
	r.gw.mark(v.Payment, circle.Failed)
	r.pass()
	rec := r.byID(v.ID)
	if rec.Status != remittance.Failed || !rec.Reversed || r.book.taken() != 1 || r.credited() != 0 {
		t.Fatalf("status %s reversed %v clawbacks %d ledger %d — the owed credit must be taken back", rec.Status, rec.Reversed, r.book.taken(), r.credited())
	}
	if len(r.book.posted()) != 1 {
		t.Fatalf("re-landing a credit that had landed posted it again (%d credits)", len(r.book.posted()))
	}
}

// A payment the ledger never took is not "clawed back" from the customer: the
// row stays open until the books can be put right, then nets to zero.
func TestAFailureWithTheLedgerDownStaysOpenUntilReversed(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	v, err := r.submit(r.pay(700, nil))
	if err != nil || !v.Credited {
		t.Fatal(err)
	}
	r.gw.mark(v.Payment, circle.Failed)
	r.book.mu.Lock()
	r.book.broken = true
	r.book.mu.Unlock()
	r.pass()
	if rec := r.byID(v.ID); rec.Status != remittance.Accepted || rec.Reversed {
		t.Fatalf("with the ledger down: %+v — must stay open, not fail with the credit standing", rec)
	}
	r.book.mu.Lock()
	r.book.broken = false
	r.book.mu.Unlock()
	r.pass()
	if rec := r.byID(v.ID); rec.Status != remittance.Failed || !rec.Reversed || r.credited() != 0 {
		t.Fatalf("after the ledger came back: %+v ledger %d", rec, r.credited())
	}
}

// R4: a row recorded and never sent expires like any other.
func TestAnUnsentRowExpires(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	sub := r.pay(100, nil)
	nets, _ := r.s.networks(context.Background())
	c, err := verify(sub, nets, treasury, r.org.Name, r.payer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.s.claim(r.org, r.payer, c); err != nil { // the process stops before drive
		t.Fatal(err)
	}
	r.now = time.Unix(int64(c.before), 0).Add(2 * time.Hour)
	r.s.Reconcile(context.Background())
	if rec := r.stored(sub); rec.Status != remittance.Failed || rec.Reason != "expired" {
		t.Fatalf("record = %+v", rec)
	}
	if settles, _ := r.gw.count(); settles != 0 {
		t.Fatal("an expired authorization was sent to Gateway")
	}
}

// R5 / M2: the options menu never waits on Gateway, and a slow failing
// Gateway is asked once per window however many callers want the table.
func TestASlowGatewayStallsNobody(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(503)
	}))
	defer srv.Close()
	now := time.Now()
	s := New(circle.Config{Network: circle.Mainnet, Treasury: treasury}, circle.New(srv.URL))
	s.now = func() time.Time { return now }

	start := time.Now()
	for i := 0; i < 5; i++ {
		if s.Networks() != nil {
			t.Fatal("offered networks no read has produced")
		}
	}
	if d := time.Since(start); d > 50*time.Millisecond || hits.Load() != 0 {
		t.Fatalf("the menu called Gateway (%d calls) or waited %v", hits.Load(), d)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.networks(context.Background()); ReasonOf(err) != ReasonUnavailable {
				t.Errorf("networks: %v", err)
			}
		}()
	}
	wg.Wait()
	if hits.Load() != 1 {
		t.Fatalf("8 concurrent readers made %d calls to Gateway, want 1", hits.Load())
	}
	// Within the failure window: remembered, not asked again.
	now = now.Add(failAge - time.Second)
	if _, err := s.networks(context.Background()); err == nil || hits.Load() != 1 {
		t.Fatalf("a remembered failure called Gateway again (%d calls)", hits.Load())
	}
	now = now.Add(2 * time.Second)
	_, _ = s.networks(context.Background())
	if hits.Load() != 2 {
		t.Fatalf("after the window Gateway was asked %d times, want 2", hits.Load())
	}
}

// The menu shows the table the rail last read, without asking Gateway.
func TestTheMenuReadsTheWarmTable(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	if r.s.Networks() != nil {
		t.Fatal("a cold rail offered networks")
	}
	r.pass() // the reconciler warms the table
	if got := r.s.Networks(); len(got) != 1 || got[0] != caip {
		t.Fatalf("warm table offers %v", got)
	}
}

// M1: two hundred rows that cannot move do not starve the one that can.
func TestStuckRowsCannotStarveTheRest(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	r.gw.set(func(f *fake) { f.down = 503 })
	nets, _ := r.s.networks(context.Background())
	claim := func() *remittance.Remittance {
		sub := r.pay(100, nil)
		c, err := verify(sub, nets, treasury, r.org.Name, r.payer)
		if err != nil {
			t.Fatal(err)
		}
		rec, err := r.s.claim(r.org, r.payer, c)
		if err != nil {
			t.Fatal(err)
		}
		return rec
	}
	for i := 0; i < batch+5; i++ {
		claim()
	}
	r.now = r.now.Add(2 * time.Minute)
	r.s.Reconcile(context.Background()) // 200 of them tried, pushed back

	fresh := claim()
	r.gw.set(func(f *fake) { f.down = 0 })
	r.now = r.now.Add(61 * time.Second)
	r.s.Reconcile(context.Background())
	if rec := r.byID(fresh.Id()); rec.Status != remittance.Accepted {
		t.Fatalf("a new row behind %d stuck ones was not reached: %+v", batch, rec)
	}
}

// L3: a refused payment handed to Gateway directly later is found by its nonce
// and credited.
func TestARefusedPaymentSettledElsewhereIsCredited(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	r.gw.set(func(f *fake) { f.held[r.from] = 0 })
	sub := r.pay(1100, nil)
	if _, err := r.submit(sub); ReasonOf(err) != "insufficient_balance" {
		t.Fatalf("submit: %v", err)
	}
	// The payer deposits and settles the same authorization at Gateway directly.
	r.gw.fund(r.from, 1100)
	sig, _ := circle.Canonical(sub.Signature)
	if st, err := circle.New(r.server.URL).Settle(context.Background(), circle.Resource{URL: "x", Description: "x", MimeType: "x"},
		r.s.requirements(r.stored(sub)), circle.Payload{Authorization: sub.Authorization, Signature: sig}); err != nil || !st.Success {
		t.Fatalf("direct settle: %+v %v", st, err)
	}
	r.pass()
	if rec := r.stored(sub); rec.Status != remittance.Accepted || !rec.Landed {
		t.Fatalf("record = %+v", rec)
	}
	if r.credited() != 1100 {
		t.Fatalf("ledger = %d", r.credited())
	}
}

// L3: a refused payment that expires unseen is not looked at again.
func TestARefusedPaymentStopsBeingSweptWhenItExpires(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	r.gw.set(func(f *fake) { f.held[r.from] = 0 })
	sub := r.pay(100, nil)
	if _, err := r.submit(sub); err == nil {
		t.Fatal("accepted")
	}
	before, _ := strconv.ParseInt(sub.Authorization.ValidBefore, 10, 64)
	r.now = time.Unix(before, 0).Add(2 * time.Hour)
	r.s.Reconcile(context.Background())
	if rec := r.stored(sub); rec.Status != remittance.Refused || rec.Due != never {
		t.Fatalf("record = %+v", rec)
	}
}

// L4: the caller gave up after Gateway LOCKED the payment and before it
// answered. The payment is pending, and the reconciler finds and credits it.
func TestACallerThatGivesUpAfterTheLockIsCreditedByTheReconciler(t *testing.T) {
	r := newRig(t, circle.Mainnet, true)
	locked, release := make(chan struct{}), make(chan struct{})
	r.gw.set(func(f *fake) { f.locked, f.release = locked, release })
	sub := r.pay(1500, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan View, 1)
	go func() {
		v, err := r.s.Submit(ctx, r.org, r.payer, sub)
		if err != nil {
			t.Errorf("submit: %v", err)
		}
		done <- v
	}()
	<-locked
	cancel()
	v := <-done
	close(release)
	if v.Status != remittance.Pending || v.Credited {
		t.Fatalf("view = %+v, want pending", v)
	}
	r.gw.set(func(f *fake) { f.locked, f.release = nil, nil })
	r.pass()
	if rec := r.stored(sub); rec.Status != remittance.Accepted || !rec.Landed || r.credited() != 1500 {
		t.Fatalf("record = %+v, ledger %d", rec, r.credited())
	}
}
