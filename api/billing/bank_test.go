package billing

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
)

// A bank-debit top-up in dollars: its attempt is bound as delayed, as the checkout does when
// ACH is on offer, so it outlives the 48 hours a card is given.
func openACH(ctx context.Context, t *testing.T, id string, cents int64) (*organization.Organization, *fakeLedger) {
	t.Helper()
	org := routingOrg(ctx, t)
	led := newFakeLedger()
	injectLedger(t, led)
	if err := pendingintent.PutQuoted(id, org.Name, "buyer", pendingintent.Topup, cents, "usd", 0, "", 3, false); err != nil {
		t.Fatal(err)
	}
	if err := pendingintent.Bind(id, pendingintent.Terms{Delayed: true}); err != nil {
		t.Fatal(err)
	}
	return org, led
}

// While the bank has not answered (processing, or waiting on microdeposits) nothing is
// credited and the attempt is neither cancelled nor given up on past a card's 48 hours; the
// credit posts once on success, whoever reports it and however often.
func TestACHSucceededCreditsExactlyOnceAndProcessingWaits(t *testing.T) {
	ctx := heldStore(t)
	org, led := openACH(ctx, t, "pi_ach_ok", 2500)
	db := datastore.New(org.Namespaced(ctx))
	d := &stripeDouble{tx: &processor.Transaction{ID: "pi_ach_ok", ProcessorRef: "ch_ach_ok", Amount: 2500, Currency: "usd", Status: "processing"}}
	useStripe(t, d)

	p, _, _ := pendingintent.Get("pi_ach_ok")
	p.CreatedAt = time.Now().Add(-4 * 24 * time.Hour) // four days, past a card's 48 hours
	if err := reconcileIntent(ctx, p); err != nil {
		t.Fatal(err)
	}
	if len(d.cancelled) != 0 || len(led.credits()) != 0 {
		t.Fatalf("a processing debit was cancelled (%v) or credited (%d)", d.cancelled, len(led.credits()))
	}

	d.tx.Status = "succeeded"
	for i := 0; i < 3; i++ { // the webhook, delivered again and again
		if err := applySettlementEvent(ctx, db, org, succeeded("pi_ach_ok", "ch_ach_ok", 2500, "usd")); err != nil {
			t.Fatalf("delivery %d: %v", i, err)
		}
	}
	p, _, _ = pendingintent.Get("pi_ach_ok")
	if err := reconcileIntent(ctx, p); err != nil { // and the reconciler behind it
		t.Fatal(err)
	}
	got := led.credits()
	if len(got) != 1 || got[0].AmountCents != 2500 || got[0].Currency != "usd" || got[0].IdempotencyKey != "ch_ach_ok" {
		t.Fatalf("want one credit of 2500 usd keyed on the charge, got %+v", got)
	}
	st, f := ReadTopupIntent(ctx, org, "buyer", "pi_ach_ok")
	if f != nil || !st.Credited || st.State != "succeeded" {
		t.Fatalf("state = %+v (%v)", st, f)
	}
}

// A debit that failed credits nothing; one that bounces AFTER its credit posted takes the
// credit back, once, however often the failure is reported.
func TestACHFailedOrReturnedLeavesNoCredit(t *testing.T) {
	ctx := heldStore(t)
	org, led := openACH(ctx, t, "pi_ach_fail", 2500)
	db := datastore.New(org.Namespaced(ctx))
	useStripe(t, &stripeDouble{tx: &processor.Transaction{ID: "pi_ach_fail", ProcessorRef: "ch_ach_fail", Amount: 2500, Currency: "usd", Status: "requires_payment_method"}})

	failure := func(charge string) *processor.WebhookEvent {
		e := &processor.WebhookEvent{ID: "evt_fail", Type: "payment.failed", Processor: processor.Stripe,
			Data: map[string]interface{}{"id": "pi_ach_fail", "status": "requires_payment_method", "amount": float64(2500), "currency": "usd", "latest_charge": charge}}
		normalizeStripeWebhookEvent(e)
		if e.Type != "payment.failed" || isSettlementEvent(e.Type) {
			t.Fatalf("a failed debit normalized to %q", e.Type)
		}
		return e
	}
	// Before any credit: nothing moves.
	for i := 0; i < 2; i++ {
		if err := reverseFailedPayment(ctx, db, org, failure("ch_ach_fail")); err != nil {
			t.Fatal(err)
		}
	}
	if len(led.credits()) != 0 || len(led.clawbacks) != 0 {
		t.Fatalf("a failed debit moved money: credits %d, clawbacks %d", len(led.credits()), len(led.clawbacks))
	}

	// After the credit posted: the failure reverses it exactly once.
	w := settledWallet(ctx, t, "pi_ach_ret", 2500, 0, "")
	ev := func() *processor.WebhookEvent {
		e := &processor.WebhookEvent{ID: "evt_ret", Type: "payment.failed", Processor: processor.Stripe,
			Data: map[string]interface{}{"id": "pi_ach_ret", "amount": float64(2500), "currency": "usd", "latest_charge": w.charge}}
		normalizeStripeWebhookEvent(e)
		return e
	}
	for i := 0; i < 3; i++ {
		if err := reverseFailedPayment(w.ctx, w.db, w.org, ev()); err != nil {
			t.Fatal(err)
		}
	}
	if w.taken() != 2500 || w.balance() != 0 {
		t.Fatalf("returned debit: taken %d, balance %d; want the credit reversed once", w.taken(), w.balance())
	}
}

// A payment that is ACH on offer is bounded: past the per-attempt limit, or in another
// currency, ACH is left out of the Payment Element, and a method the account cannot take is
// left out too.
func TestACHIsBoundedPerAttemptAndOnlyWhereTheAccountTakesIt(t *testing.T) {
	ctx := context.Background()
	pol := routing.Default()
	has := func(list []string, m string) bool {
		for _, x := range list {
			if x == m {
				return true
			}
		}
		return false
	}
	cap := &capableStripe{have: map[string]bool{"us_bank_account": true, "pix": true}}
	ex, ach := bankExclusions(ctx, cap, pol, "usd", 10000)
	if !ach || has(ex, "us_bank_account") || !has(ex, "sepa_debit") || has(ex, "pix") {
		t.Fatalf("a $100 dollar payment: ach=%v excluded=%v", ach, ex)
	}
	if ex, ach = bankExclusions(ctx, cap, pol, "usd", routing.ACHMaxCents+1); ach || !has(ex, "us_bank_account") {
		t.Fatalf("over the limit ACH stayed: ach=%v %v", ach, ex)
	}
	if ex, ach = bankExclusions(ctx, cap, pol, "brl", 10000); ach || !has(ex, "us_bank_account") {
		t.Fatalf("ACH offered in reais: %v", ex)
	}
	if ex, ach = bankExclusions(ctx, &capableStripe{have: map[string]bool{}}, pol, "usd", 10000); ach || !has(ex, "us_bank_account") || !has(ex, "pix") {
		t.Fatalf("a method the account cannot take was offered: %v", ex)
	}
}

type capableStripe struct {
	stripeDouble
	have map[string]bool
}

func (c *capableStripe) Methods(context.Context) map[string]bool { return c.have }

// A local bank method (SEPA debit, in euros) settles once, for exactly the quote.
func TestALocalBankMethodSettlesOnce(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	led := newFakeLedger()
	injectLedger(t, led)
	if err := pendingintent.PutQuoted("pi_sepa", org.Name, "buyer", pendingintent.Topup, 1000, "usd", 920, "eur", 3, false); err != nil {
		t.Fatal(err)
	}
	if err := pendingintent.Bind("pi_sepa", pendingintent.Terms{Rate: 0.92, RateSource: "processor", Delayed: true}); err != nil {
		t.Fatal(err)
	}
	useStripe(t, &stripeDouble{tx: &processor.Transaction{ID: "pi_sepa", ProcessorRef: "ch_sepa", Amount: 920, Currency: "eur", Status: "succeeded"}})
	db := datastore.New(org.Namespaced(ctx))
	for i := 0; i < 3; i++ {
		if err := applySettlementEvent(ctx, db, org, succeeded("pi_sepa", "ch_sepa", 920, "eur")); err != nil {
			t.Fatal(err)
		}
	}
	p, _, _ := pendingintent.Get("pi_sepa")
	if err := reconcileIntent(ctx, p); err != nil {
		t.Fatal(err)
	}
	if got := led.credits(); len(got) != 1 || got[0].AmountCents != 1000 {
		t.Fatalf("want one credit of the 1000 usd quoted, got %+v", got)
	}
}

// A wire carries its own reference, is credited when an admin (or a bank feed) reconciles
// it, once, for the amount first credited, and every reconcile is recorded.
func TestWireReconcileIsIdempotentAndAudited(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	led := newFakeLedger()
	injectLedger(t, led)

	ref, err := openWireReference(org, "buyer")
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := openWireReference(org, "buyer"); again != ref {
		t.Fatalf("an open wire changed its reference: %s -> %s", ref, again)
	}
	if other, _ := openWireReference(org, "someone-else"); other == ref {
		t.Fatal("two payers share a wire reference")
	}

	if _, f := ReconcileWire(ctx, "WIRE-NOBODY", 5000, "bank-1", "z"); f == nil || f.Status != 404 {
		t.Fatalf("a reference nobody was given credited: %v", f)
	}
	if _, f := ReconcileWire(ctx, ref, 5000, "bank-1", ""); f == nil {
		t.Fatal("an anonymous reconcile credited")
	}
	for i := 0; i < 3; i++ {
		out, f := ReconcileWire(ctx, ref, 5000, "bank-1", "z@hanzo.ai")
		if f != nil {
			t.Fatalf("reconcile %d: %v", i, f.Message)
		}
		if out.Credited != (i == 0) {
			t.Fatalf("reconcile %d credited=%v", i, out.Credited)
		}
	}
	if _, f := ReconcileWire(ctx, ref, 7000, "bank-2", "z@hanzo.ai"); f == nil || f.Status != 409 {
		t.Fatalf("a different amount for a credited wire: %v", f)
	}
	got := led.credits()
	if len(got) != 1 || got[0].AmountCents != 5000 || got[0].Subject != "buyer" || got[0].IdempotencyKey != "wire:"+ref {
		t.Fatalf("want one credit of 5000 for the payer, got %+v", got)
	}
	// Credited, the reference is spent: the next wire gets a new one.
	if next, _ := openWireReference(org, "buyer"); next == ref {
		t.Fatal("a credited wire's reference was handed out again")
	}
}

// Every bank method is listed only to buyers in its own countries paying in its own currency:
// ACH to the US in dollars, SEPA to the EEA in euros, Pix to Brazil in reais, and so on. The
// currency is the one the buyer chose, else their country's own.
func TestEachBankMethodIsListedOnlyInItsOwnCountriesAndCurrency(t *testing.T) {
	ctx := context.Background()
	all := map[string]bool{}
	for _, m := range routing.BankMethodNames() {
		all[m] = true
	}
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(&capableStripe{stripeDouble: stripeDouble{}, have: all})
	// When no Square country rules own these markets, Stripe lists each country's bank methods.
	pol := routing.Default()
	pol.CountryRules = nil
	pol.LocalCurrencies = nil
	for _, tc := range []struct {
		country, cur string // cur "" is the country's own
		want         string // sorted, comma-joined
	}{
		{"US", "", "ach"}, {"US", "usd", "ach"}, {"US", "eur", ""}, {"US", "cad", ""},
		{"DE", "", "sepa_debit"}, {"DE", "eur", "sepa_debit"}, {"DE", "usd", ""},
		{"IN", "", "upi"}, {"IN", "inr", "upi"}, {"IN", "usd", ""}, {"IN", "eur", ""},
		{"BR", "", "pix"}, {"BR", "brl", "pix"}, {"BR", "usd", ""},
		{"NL", "", "ideal,sepa_debit"}, {"NL", "usd", ""},
		{"BE", "", "bancontact,sepa_debit"}, {"BE", "usd", ""},
		{"PL", "", "blik,p24"}, {"PL", "eur", "p24,sepa_debit"}, {"PL", "usd", ""},
		{"AT", "", "eps,sepa_debit"}, {"AT", "usd", ""},
		{"PT", "", "multibanco,sepa_debit"}, {"PT", "usd", ""},
		{"CA", "", "acss_debit"}, {"CA", "cad", "acss_debit"}, {"CA", "usd", ""},
		{"FR", "", "sepa_debit"}, {"IE", "", "sepa_debit"}, {"NO", "eur", "sepa_debit"},
		{"GB", "", "bacs_debit"}, {"GB", "eur", ""}, {"AU", "", "au_becs_debit"}, {"MY", "", "fpx"},
		// Outside every one of them.
		{"SE", "", ""}, {"SE", "eur", "sepa_debit"}, {"CH", "", ""}, {"CH", "eur", ""}, {"JP", "", ""}, {"MX", "", ""}, {"ZA", "", ""},
	} {
		got := []string{}
		for _, a := range bankAlternatives(ctx, nil, reg, pol, tc.country, pol.CheckoutCurrency(tc.country, tc.cur)) {
			if a != "wire" {
				got = append(got, a)
			}
		}
		sort.Strings(got)
		if g := strings.Join(got, ","); g != tc.want {
			t.Errorf("%s paying in %q: listed %q; want %q", tc.country, tc.cur, g, tc.want)
		}
	}
	// A method the account cannot take is not listed, in its own country.
	off := processor.NewRegistry(processor.DefaultConfig())
	off.Register(&capableStripe{stripeDouble: stripeDouble{}, have: map[string]bool{}})
	if got := bankAlternatives(ctx, nil, off, pol, "US", "usd"); len(got) != 0 {
		t.Errorf("a US buyer on an account with no bank capability is offered %v", got)
	}
	// Under the default Square-first policy, Stripe bank methods are never surfaced in
	// any of Square's eight markets (US, CA, AU, GB, JP, FR, IE, ES), while Square ACH
	// is surfaced in the US when Square is available.
	defPol := routing.Default()
	for _, c := range routing.SquareCountries {
		if got := bankAlternatives(ctx, nil, reg, defPol, c, defPol.CheckoutCurrency(c, "")); len(got) != 0 {
			t.Errorf("default Square policy surfaced Stripe bank methods %v in %s", got, c)
		}
	}
	regWithSquare := processor.NewRegistry(processor.DefaultConfig())
	regWithSquare.Register(newRail(processor.Square, true))
	regWithSquare.Register(&capableStripe{stripeDouble: stripeDouble{}, have: all})
	if got := bankAlternatives(ctx, nil, regWithSquare, defPol, "US", "usd"); strings.Join(got, ",") != "ach" {
		t.Errorf("US with Square available listed %v; want [ach]", got)
	}
	for _, c := range []string{"CA", "AU", "GB", "JP", "FR", "IE", "ES"} {
		if got := bankAlternatives(ctx, nil, regWithSquare, defPol, c, "usd"); len(got) != 0 {
			t.Errorf("%s with Square available listed Stripe bank methods %v", c, got)
		}
	}
}
