package billing

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/routingpolicy"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
)

// quotingRail is a Stripe that quotes its own rate, as one with FX Quotes does.
type quotingRail struct {
	rail
	rq    processor.RateQuote
	fail  bool
	locks []string
}

func (r *quotingRail) QuoteRate(_ context.Context, cur, lock string) (*processor.RateQuote, error) {
	r.locks = append(r.locks, lock)
	if r.fail {
		return nil, errors.New("fx quotes are not enabled for this account")
	}
	q := r.rq
	q.Currency, q.Lock = cur, lock
	return &q, nil
}

func useQuotingRail(t *testing.T, r *quotingRail) *processor.Registry {
	t.Helper()
	shownMu.Lock()
	shown = map[string]shownRate{}
	shownMu.Unlock()
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.SetConfig(&processor.RegistryConfig{ProcessorPriority: []processor.ProcessorType{processor.Stripe}, DisabledProcessors: map[processor.ProcessorType]bool{}})
		reg.Register(r)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old; shownMu.Lock(); shown = map[string]shownRate{}; shownMu.Unlock() })
	return processorsForOrg(nil)
}

func TestStripesRateIsPassedThroughOrAbsorbedAndBoundToThePayment(t *testing.T) {
	ctx := heldStore(t)
	stubRates(t, map[string]float64{"inr": 90})
	r := &quotingRail{rail: newRail(processor.Stripe, true), rq: processor.RateQuote{ID: "fxq_1", PerDollar: 95.2361, BasePerDollar: 94.2842,
		ExchangeRate: 0.0105002, FeeRate: 0.01, Premium: 0.001, ReferenceRate: 0.0106108, ReferenceProvider: "ecb", ExpiresAt: 1900000000}}
	reg := useQuotingRail(t, r)

	// Pass (the default): $8 at Stripe's effective rate, fee and premium in it: 761.89 -> Rs 762.
	p := routing.Default()
	q, f := quoteFor(ctx, reg, p, 800, "IN", "", "", false)
	if f != nil || q.Amount != 76200 || q.Source != "processor" || q.FXFee != "pass" || q.Rate != 95.2361 || q.QuoteID != "" || q.ExchangeRate != 0.0105002 {
		t.Fatalf("pass: %+v %v", q, f)
	}
	if len(r.locks) != 1 || r.locks[0] != "none" {
		t.Fatalf("a displayed price takes the current rate with no lock: %v", r.locks)
	}
	// A displayed price reuses the cached rate; it does not quote again.
	quoteFor(ctx, reg, p, 800, "IN", "", "", false)
	if len(r.locks) != 1 {
		t.Fatalf("the shown rate was not cached: %v", r.locks)
	}
	// Absorb: base_rate, Hanzo bears the cost: 754.27 -> Rs 755.
	p.FXFee = routing.FXAbsorb
	if q, _ = quoteFor(ctx, reg, p, 800, "IN", "", "", false); q.Amount != 75500 || q.FXFee != "absorb" {
		t.Fatalf("absorb: %+v", q)
	}
	// A payment being opened locks the rate for an hour, a voucher method for a day, and
	// the quote names the lock the intent is bound to.
	p = routing.Default()
	if q, _ = quoteFor(ctx, reg, p, 800, "IN", "", "", true); q.QuoteID != "fxq_1" || q.ReferenceProvider != "ecb" || q.LockExpiresAt != 1900000000 || r.locks[len(r.locks)-1] != "hour" {
		t.Fatalf("bind: %+v %v", q, r.locks)
	}
	r.rq.PerDollar, r.rq.BasePerDollar = 5.1, 5.05
	if q, _ = quoteFor(ctx, reg, p, 1000, "BR", "", "pix", true); q.Amount != 5100 || r.locks[len(r.locks)-1] != "day" {
		t.Fatalf("pix locks a day: %+v %v", q, r.locks)
	}
	// Without FX Quotes the market rate is used, no margin, and the quote says Hanzo
	// bears the conversion cost.
	r.fail = true
	if q, f = quoteFor(ctx, reg, p, 800, "IN", "", "", true); f != nil || q.Source != "reference" || q.FXFee != "absorb" || q.Amount != 72000 || q.QuoteID != "" {
		t.Fatalf("fallback: %+v %v", q, f)
	}
	// The market source can be chosen outright.
	r.fail = false
	p.FXSource = routing.FXReference
	if q, _ = quoteFor(ctx, reg, p, 800, "IN", "", "", false); q.Source != "reference" || q.Amount != 72000 {
		t.Fatalf("market: %+v", q)
	}
}

// A route to Square or to crypto yields a valid quote, and never asks Stripe for a rate.
func TestARouteToSquareOrCryptoQuotesWithoutAskingStripe(t *testing.T) {
	ctx := heldStore(t)
	stripe := &quotingRail{rail: newRail(processor.Stripe, true), rq: processor.RateQuote{ID: "fxq_x", PerDollar: 95, BasePerDollar: 94}}
	old := processorsForOrg
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.SetConfig(&processor.RegistryConfig{ProcessorPriority: []processor.ProcessorType{processor.Square, processor.Stripe}, DisabledProcessors: map[processor.ProcessorType]bool{}})
	reg.Register(newRail(processor.Square, true))
	reg.Register(stripe)
	processorsForOrg = func(*organization.Organization) *processor.Registry { return reg }
	t.Cleanup(func() { processorsForOrg = old })
	stubRates(t, map[string]float64{"inr": 94.25})

	p := routing.Default()
	for _, via := range []processor.ProcessorType{processor.Square, "crypto"} {
		q, f := quoteVia(ctx, reg, p, via, 2000, "IN", "", "", true)
		if f != nil || q.Currency != "usd" || q.Amount != 2000 || q.UsdCents != 2000 || q.Rate != 1 || q.Source != "none" || q.Decimals != 2 {
			t.Fatalf("%s: %+v %v", via, q, f)
		}
	}
	if q := nativeQuote(2000); q.Source != "none" || q.Rate != 1 || q.Amount != 2000 {
		t.Fatalf("native: %+v", q)
	}
	if len(stripe.locks) != 0 {
		t.Fatalf("Stripe was asked for a rate: %v", stripe.locks)
	}
	// A rail without a quote of its own that can take the currency prices on the
	// reference rate: the same path, another source.
	q, f := quoteVia(ctx, reg, p, processor.Stripe, 800, "IN", "", "", false)
	if f != nil || q.Source != "processor" || len(stripe.locks) != 1 {
		t.Fatalf("stripe: %+v %v", q, f)
	}
	p.FXSource = routing.FXReference
	if q, f = quoteVia(ctx, reg, p, processor.Stripe, 800, "IN", "", "", false); f != nil || q.Source != "reference" || q.Amount != 75400 || len(stripe.locks) != 1 {
		t.Fatalf("reference: %+v %v", q, f)
	}
}

// A policy stored before processors and accounts held its split in columns. It is read
// once into the processors it meant, rewritten in the new shape, and read the same
// afterwards; its saved versions are read the same way, and restore as the new shape.
func TestAPolicyStoredInTheOldShapeIsMigratedOnceAtLoad(t *testing.T) {
	ctx := heldStore(t)
	useRails(t, true, true)
	scope := "legacy-" + t.Name()
	old := routingpolicy.Row{SquareWeight: 70, SquareOn: true, StripeOn: true, PayPalOn: true, CryptoOn: false, LocalOn: true,
		Rules: `{"fallback":["stripe"],"noSticky":true}`, Version: 3, By: "z@hanzo.ai"}
	if err := routingpolicy.Put(ctx, scope, old); err != nil {
		t.Fatal(err)
	}
	want := []routing.ProcessorRule{{Processor: "square", On: true, Weight: 70}, {Processor: "stripe", On: true, Weight: 30}, {Processor: "paypal", On: true}, {Processor: "crypto"}, {Processor: "kraken"}, {Processor: "mpc"}}
	p, v, err := ReadRoutingVersioned(ctx, scope)
	if err != nil || v != 3 || !reflect.DeepEqual(p.Processors, want) || len(p.Fallback) != 1 || p.Sticky {
		t.Fatalf("migrated policy = %+v (v%d, %v); want %+v, the rules kept", p, v, err, want)
	}
	// Written back in the new shape: the columns are gone, the split is in the rules.
	row, found, _ := routingpolicy.Get(ctx, scope)
	if !found || row.SquareWeight != 0 || row.SquareOn || row.StripeOn || row.PayPalOn || !strings.Contains(row.Rules, `"processors"`) || row.Version != 3 || row.By != "z@hanzo.ai" {
		t.Fatalf("row after the migration = %+v; want the new shape with its version and author", row)
	}
	again, _, _ := ReadRoutingVersioned(ctx, scope)
	if !reflect.DeepEqual(again.Processors, want) {
		t.Fatalf("a second read = %+v", again.Processors)
	}

	// A version saved in the old shape reads as the processors it meant, and restores as
	// a new version in the new shape.
	if err := routingpolicy.PutVersion(ctx, routingpolicy.Version{Scope: scope, N: 1, By: "z", Note: "old",
		Policy: `{"squareWeight":20,"squareOn":true,"stripeOn":true,"paypalOn":true,"cryptoOn":true,"localOn":true,"currencyRules":[],"countryRules":[],"brandRules":[],"issuerCountryRules":[],"fallback":[],"methods":[],"fxSpreadBps":200,"localCurrencies":[]}`}); err != nil {
		t.Fatal(err)
	}
	vs, _ := ListRoutingVersions(ctx, scope)
	if len(vs) != 1 || squareWeightOf(vs[0].Policy) != 20 || len(vs[0].Policy.Processors) != 6 {
		t.Fatalf("the old version reads %+v", vs)
	}
	restored, f := RestoreRouting(ctx, scope, 1, "z", "")
	if f != nil || squareWeightOf(restored) != 20 || restored.Processors[1].Weight != 80 {
		t.Fatalf("restore = %+v %v; want Square 20, Stripe 80", restored, f)
	}
	if row, _, _ := routingpolicy.Get(ctx, scope); !strings.Contains(row.Rules, `"processors"`) || row.SquareWeight != 0 {
		t.Fatalf("the restored row is in the old shape: %+v", row)
	}
}

func TestCryptoIsOfferedOnlyWhenTheCryptoMenuCanServe(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	t.Cleanup(processor.SetChargePolicy(func() map[processor.ProcessorType]bool { return processor.ChargeNames("stripe,square,paypal") }))
	// Custody only: the rails send no stablecoin to Stripe, so crypto is offered only if the menu serves.
	custody := routing.Default()
	custody.CryptoRails = []routing.CryptoRoute{{Asset: "*", Processor: "mpc"}}
	if _, f := WriteRouting(ctx, org.Name, custody, "z", ""); f != nil {
		t.Fatal(f.Err)
	}
	old, oldServes := processorsForOrg, cryptoServes
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		for _, k := range []processor.ProcessorType{processor.Square, processor.Stripe, processor.PayPal} {
			reg.Register(newRail(k, true))
		}
		return reg
	}
	t.Cleanup(func() { processorsForOrg, cryptoServes = old, oldServes })

	// The picker cannot serve (custody signer not allowed or unreachable): crypto is not offered.
	cryptoServes = func(context.Context) bool { return false }
	if d, f := Place(ctx, org, RouteIn{Subject: "c", Country: "DE"}); f != nil || contains(d.Alternatives, "crypto") {
		t.Fatalf("crypto was offered while the menu cannot serve: %+v %v", d, f)
	}
	// The picker can serve: crypto is offered beside the card.
	cryptoServes = func(context.Context) bool { return true }
	if d, f := Place(ctx, org, RouteIn{Subject: "c", Country: "DE"}); f != nil || !contains(d.Alternatives, "crypto") {
		t.Fatalf("crypto was not offered while the menu can serve: %+v %v", d, f)
	}
	// The real answer with the custody processors not allowed by the charge policy is "cannot serve".
	cryptoServes = oldServes
	if cryptoServes(ctx) {
		t.Fatal("crypto served with mpc and kraken outside the policy's processors")
	}
	// With Stripe first for USDC crypto is the Payment Element's, and offered whatever the custody menu says.
	cryptoServes = func(context.Context) bool { return false }
	if _, f := WriteRouting(ctx, org.Name, routing.Default(), "z", ""); f != nil {
		t.Fatal(f.Err)
	}
	if d, f := Place(ctx, org, RouteIn{Subject: "c2", Country: "DE"}); f != nil || !contains(d.Alternatives, "crypto") {
		t.Fatalf("Stripe crypto was not offered: %+v %v", d, f)
	}
}

func contains(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

// A saved card holds the buyer to its processor only while that processor can charge the
// checkout's currency: a saved Square card in euros, rupees or reais routes to Stripe, in
// dollars it stays on Square, and the same rule answers the top-up intent.
func TestASavedCardOnlyHoldsTheBuyerWhereItsProcessorCanCharge(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	useRails(t, true, true)
	db := datastore.New(org.Namespaced(ctx))
	pm := paymentmethod.New(db)
	pm.CustomerId, pm.Type, pm.ProviderType, pm.IsDefault = "holder", "card", "square", true
	if err := pm.Create(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ country, cur string }{{"DE", "eur"}, {"IN", "inr"}, {"BR", "brl"}} {
		d, f := Place(ctx, org, RouteIn{Subject: "holder", Country: tc.country, Currency: tc.cur})
		if f != nil || d.Processor != processor.Stripe || d.Reason == "saved" {
			t.Fatalf("a saved Square card paying in %s: %+v %v; want Stripe, the card not offered", tc.cur, d, f)
		}
	}
	if d, f := Place(ctx, org, RouteIn{Subject: "holder", Country: "US", Currency: "usd"}); f != nil || d.Processor != processor.Square || d.Reason != "saved" {
		t.Fatalf("a saved Square card paying in dollars stays on Square: %+v %v", d, f)
	}
	// The top-up intent follows the same rule: a euro top-up is Stripe's, not a 503.
	if _, f := OpenTopupIntent(ctx, org, TopupIntentIn{Subject: "holder", Country: "DE", AmountCents: 500, Currency: "eur"}); f != nil && f.Status == 503 && strings.Contains(f.Message, "saved card") {
		t.Fatalf("a euro top-up with a saved Square card answered the saved card's 503: %v", f.Message)
	}
	// Choosing ACH while holding a saved Square card routes to Stripe (never 409 "this checkout is on square").
	for _, m := range []string{"ach", "us_bank_account"} {
		d, f := Place(ctx, org, RouteIn{Subject: "holder", Country: "US", Currency: "usd", Method: m})
		if f != nil || d.Processor != processor.Stripe || d.Reason != "method" {
			t.Fatalf("a saved Square card paying via %s: %+v %v; want Stripe (method)", m, d, f)
		}
	}
	if _, f := OpenTopupIntent(ctx, org, TopupIntentIn{Subject: "holder", Country: "US", AmountCents: 2000, Method: "ach"}); f != nil && f.Status == 409 {
		t.Fatalf("an ACH top-up with a saved Square card hit 409: %v", f.Message)
	}
	// When Square is switched off in policy (100% Stripe), a buyer with a saved Square card
	// falls through to Stripe rather than getting routed to an unmounted Square card form.
	stripeOnly := withWeight(0)
	setOn(&stripeOnly, "square", false)
	if _, f := WriteRouting(ctx, org.Name, stripeOnly, "z", "100% stripe"); f != nil {
		t.Fatal(f.Err)
	}

	// Choosing ACH while holding a saved Stripe card routes to Stripe (never 422 "local payment methods not offered").
	// Change the saved card to Stripe.
	pm.ProviderType = "stripe"
	if err := pm.Create(); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"ach", "us_bank_account"} {
		d, f := Place(ctx, org, RouteIn{Subject: "holder", Country: "US", Currency: "usd", Method: m})
		if f != nil || d.Processor != processor.Stripe || d.Reason != "method" {
			t.Fatalf("a saved Stripe card paying via %s: %+v %v; want Stripe (method)", m, d, f)
		}
	}
	if _, f := OpenTopupIntent(ctx, org, TopupIntentIn{Subject: "holder", Country: "US", AmountCents: 2000, Method: "ach"}); f != nil && f.Status == 422 {
		t.Fatalf("an ACH top-up with a saved Stripe card hit 422: %v", f.Message)
	}
	// Reset to saved Square card for the next test
	pm.ProviderType = "square"
	if err := pm.Create(); err != nil {
		t.Fatal(err)
	}

	if d, f := Place(ctx, org, RouteIn{Subject: "holder", Country: "US", Currency: "usd"}); f != nil || d.Processor != processor.Stripe || d.Reason == "saved" {
		t.Fatalf("with Square switched off, a saved Square card holder routed to %+v %v; want Stripe", d, f)
	}
}

func TestReadRoutingViewReportsHealthySwitchedOffSquareAvailable(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	useRails(t, true, true)
	t.Cleanup(processor.SetChargePolicy(func() map[processor.ProcessorType]bool {
		return processor.ChargeNames("stripe,paypal")
	}))
	v, f := ReadRoutingView(ctx, org, org.Name)
	if f != nil {
		t.Fatal(f.Err)
	}
	found := false
	for _, st := range v.Processor {
		if st.Processor == "square" {
			found = true
			if !st.Available {
				t.Fatalf("expected healthy registered Square processor to report available=true even when switched off in charge policy")
			}
		}
	}
	if !found {
		t.Fatalf("square processor missing from ReadRoutingView: %+v", v.Processor)
	}
}
