package billing

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/autorecharge"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// ─── one low-balance trigger, one charge ─────────────────────────────────────

// withProcessor points the saved-card processor seam at p for the test.
func withProcessor(t *testing.T, p processor.PaymentProcessor) {
	t.Helper()
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(p)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
}

// keyedSquare answers a key the way Square's adapter did before a reused key was
// reported as unknown: the same key with the same amount replays the first payment,
// the same key with another amount is a refusal (processor.Answer). The first charge
// settles and its answer is lost.
type keyedSquare struct {
	*MockSquareProcessor
	seen map[string]int64
	lost bool
}

func (s *keyedSquare) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if amt, ok := s.seen[req.IdempotencyKey]; ok && amt != int64(req.Amount) {
		err := &processor.Answer{Err: errors.New("INVALID_REQUEST_ERROR: IDEMPOTENCY_KEY_REUSED")}
		return &processor.PaymentResult{Success: false, Error: err, ErrorMessage: err.Error()}, err
	}
	s.seen[req.IdempotencyKey] = int64(req.Amount)
	res, err := s.MockSquareProcessor.Charge(ctx, req)
	if !s.lost {
		s.lost = true
		return nil, context.DeadlineExceeded
	}
	return res, err
}

// A charge whose answer was lost, then the rule's amount edited: the trigger is
// asked again with the amount it was reserved with, so the processor settles it
// once and that one charge is credited.
func TestAnEditAfterALostAnswerIsNotASecondCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "guard-reused", 2500, 1000000)
	m := squareMock("", "", "sqpay_reused")
	withProcessor(t, &keyedSquare{MockSquareProcessor: m, seen: map[string]int64{}})
	db := datastore.New(org.Namespaced(ctx))

	rechargeRun(t, ctx) // settles 2500, answer lost
	if _, _, err := changeRule(db, org.Name, func(c *autorecharge.AutoRecharge) bool { c.AmountCents = 3000; return true }); err != nil {
		t.Fatal(err)
	}
	rechargeRun(t, ctx)
	rechargeRun(t, ctx)
	if m.chargeCalls != 1 {
		t.Fatalf("one trigger settled %d charges at the processor", m.chargeCalls)
	}
	if got := balanceOf(t, ctx, org, org.Name); got != 2500 {
		t.Fatalf("the settled charge credited %d, want 2500", got)
	}
}

// lostFirst loses the answer to the first charge it settles.
type lostFirst struct {
	*MockSquareProcessor
	lost bool
}

func (l *lostFirst) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	res, err := l.MockSquareProcessor.Charge(ctx, req)
	if !l.lost {
		l.lost = true
		return nil, context.DeadlineExceeded
	}
	return res, err
}

// A charge whose answer was lost, then a new default card: the trigger is asked
// again on the card it was reserved with, never the new one, and the processor —
// which refuses a key asked for anything else — settles it once.
func TestANewCardAfterALostAnswerIsNotASecondCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "guard-card", 2500, 1000000)
	m := squareMock("", "", "sqpay_card")
	withProcessor(t, &lostFirst{MockSquareProcessor: m})
	db := datastore.New(org.Namespaced(ctx))

	rechargeRun(t, ctx) // settles on the first card, answer lost
	first := paymentmethod.New(db)
	if err := first.GetById(defaultPaymentMethod(db, org.Name).Id()); err != nil {
		t.Fatal(err)
	}
	first.IsDefault = false
	if err := first.Update(); err != nil {
		t.Fatal(err)
	}
	seedSavedCard(t, db, org.Name, "ccof_second", "cust_second")
	rechargeRun(t, ctx)
	if m.chargeCalls != 1 || m.lastChargeToken != first.ProviderRef {
		t.Fatalf("the trigger was charged %d times, last on %s; want once on %s", m.chargeCalls, m.lastChargeToken, first.ProviderRef)
	}
	if got := balanceOf(t, ctx, org, org.Name); got != 2500 {
		t.Fatalf("the settled charge credited %d, want 2500", got)
	}
	if got := storedRule(t, ctx, org); got.Seq != 1 || got.Amount != 0 {
		t.Fatalf("the trigger was not closed: seq %d, open for %d", got.Seq, got.Amount)
	}
}

// readFails fails the first balance read after a credit, as a transient ledger read
// does.
type readFails struct {
	*fakeLedger
	mu       sync.Mutex
	credited bool
	failed   bool
}

func (f *readFails) Credit(ctx context.Context, in creditledger.CreditInput) (string, int64, error) {
	f.mu.Lock()
	f.credited = true
	f.mu.Unlock()
	return f.fakeLedger.Credit(ctx, in)
}

func (f *readFails) Balance(ctx context.Context, org, subject, cur string, test bool) (int64, error) {
	f.mu.Lock()
	fail := f.credited && !f.failed
	if fail {
		f.failed = true
	}
	f.mu.Unlock()
	if fail {
		return 0, errors.New("ledger: read timed out")
	}
	return f.fakeLedger.Balance(ctx, org, subject, cur, test)
}

// A balance read that fails right after a reload changes nothing on the rule, and
// a wallet drained below its threshold afterwards is reloaded.
func TestAFailedReadAfterAReloadDoesNotEndReloads(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	led := &readFails{fakeLedger: newFakeLedger()}
	injectLedger(t, led)
	org := payerOrg(t, ctx, "hanzo")
	payerRule(t, ctx, org, "hanzo/erin", 2500, 1000)
	m := squareMock("", "", "sqpay_read")
	withFakeSquare(t, m)

	rechargeRun(t, ctx)
	if _, _, err := led.Debit(ctx, creditledger.DebitInput{Org: "hanzo", Subject: "hanzo/erin", Currency: "usd", Ref: "use-1", AmountCents: 2000}); err != nil {
		t.Fatal(err)
	}
	m.chargeRef = "sqpay_read_2"
	rechargeRun(t, ctx)
	if m.chargeCalls != 2 {
		got := loadAutoRecharge(datastore.New(org.Namespaced(ctx)), "hanzo/erin")
		t.Fatalf("drained to $5 below a $10 threshold and not reloaded: card calls %d, rule seq=%d after=%d", m.chargeCalls, got.Seq, got.After)
	}
}

// A rule row keyed by a subject that is not an account of the org — the per-user
// key rules once carried — is never charged by the sweep.
func TestARuleKeyedOutsideTheOrgIsNotSwept(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := payerOrg(t, ctx, "acme")
	payerRule(t, ctx, org, "usr_legacy_9f2", 2500, 1000000)
	m := squareMock("", "", "sqpay_legacy")
	withFakeSquare(t, m)
	rechargeRun(t, ctx)
	if m.chargeCalls != 0 {
		t.Fatalf("the sweep charged a card for a rule keyed %q, outside acme's accounts", "usr_legacy_9f2")
	}
}

// Sweeps running at once on one trigger charge and credit it once.
func TestOverlappingSweepsChargeOneTriggerOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "guard-overlap", 2500, 1000000)
	m := squareMock("", "", "sqpay_overlap4")
	withFakeSquare(t, m)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = RunAutoRecharge(context.Context(ctx), nil) }()
	}
	wg.Wait()
	if m.chargeCalls != 1 || balanceOf(t, ctx, org, org.Name) != 2500 {
		t.Fatalf("4 overlapping sweeps: %d card calls, balance %d; want 1 and 2500", m.chargeCalls, balanceOf(t, ctx, org, org.Name))
	}
}

// inside runs sweeps inside the first balance read it answers, and answers that
// read with the balance as it stood before them: the read of a sweep that others
// overtake.
type inside struct {
	*fakeLedger
	mu    sync.Mutex
	sweep func()
	done  bool
}

func (i *inside) Balance(ctx context.Context, org, subject, cur string, test bool) (int64, error) {
	bal, err := i.fakeLedger.Balance(ctx, org, subject, cur, test)
	i.mu.Lock()
	run := !i.done
	i.done = true
	i.mu.Unlock()
	if run {
		i.sweep()
	}
	return bal, err
}

// A sweep that read the balance before other sweeps reloaded and closed the
// trigger charges nothing: the trigger it saw is no longer the rule's.
func TestASweepOvertakenByAReloadChargesNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	led := &inside{fakeLedger: newFakeLedger()}
	injectLedger(t, led)
	org := payerOrg(t, ctx, "hanzo")
	payerRule(t, ctx, org, "hanzo/gus", 2500, 1000)
	m := squareMock("", "", "sqpay_inside")
	withFakeSquare(t, m)
	led.sweep = func() {
		for range 3 {
			_, _ = RunAutoRecharge(context.Context(ctx), nil)
		}
	}

	rechargeRun(t, ctx)
	if m.chargeCalls != 1 || len(led.credits()) != 1 {
		t.Fatalf("one trigger under overtaking sweeps: %d card calls, %d credits; want 1 and 1", m.chargeCalls, len(led.credits()))
	}
}

// failsOnce refuses the first credit under each key it is asked to fail.
type failsOnce struct {
	*fakeLedger
	mu   sync.Mutex
	keys map[string]bool
}

func (f *failsOnce) Credit(ctx context.Context, in creditledger.CreditInput) (string, int64, error) {
	f.mu.Lock()
	fail := f.keys[in.IdempotencyKey]
	delete(f.keys, in.IdempotencyKey)
	f.mu.Unlock()
	if fail {
		return "", 0, errors.New("ledger: unavailable")
	}
	return f.fakeLedger.Credit(ctx, in)
}

// A reload charged and not credited is credited on the next sweep, by its own
// processor reference, even when a top-up has since lifted the balance over the
// threshold — the top-up is not the reload's credit.
func TestAnUncreditedReloadIsCreditedOnTheNextSweep(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	led := &failsOnce{fakeLedger: newFakeLedger(), keys: map[string]bool{"sqpay_owed": true}}
	injectLedger(t, led)
	org := payerOrg(t, ctx, "hanzo")
	payerRule(t, ctx, org, "hanzo/fay", 2500, 1000)
	m := squareMock("", "", "sqpay_owed")
	withFakeSquare(t, m)

	if run := rechargeRun(t, ctx); run.Charged != 0 || m.chargeCalls != 1 {
		t.Fatalf("the first sweep = %+v (card calls %d), want one charge not yet credited", run, m.chargeCalls)
	}
	if _, _, err := led.Credit(ctx, creditledger.CreditInput{Org: "hanzo", Subject: "hanzo/fay", Currency: "usd", IdempotencyKey: "topup-by-hand", AmountCents: 5000, Cash: true}); err != nil {
		t.Fatal(err)
	}
	if run := rechargeRun(t, ctx); run.Charged != 1 || m.chargeCalls != 1 {
		t.Fatalf("the next sweep = %+v (card calls %d), want the reload credited and nothing charged", run, m.chargeCalls)
	}
	refs := map[string]int64{}
	for _, in := range led.credits() {
		refs[in.IdempotencyKey] = in.AmountCents
	}
	if refs["sqpay_owed"] != 2500 || refs["topup-by-hand"] != 5000 {
		t.Fatalf("credits %v, want the reload's 2500 under its reference beside the top-up", refs)
	}
	if bal, _ := led.Balance(ctx, "hanzo", "hanzo/fay", "usd", false); bal != 7500 {
		t.Fatalf("fay's wallet holds %d, want 7500", bal)
	}
}
