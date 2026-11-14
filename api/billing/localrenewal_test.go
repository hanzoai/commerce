package billing

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
	"github.com/hanzoai/commerce/util/test/ae"
)

func TestARenewalIsRequotedAndTheChangeIsAnnounced(t *testing.T) {
	ctx := heldStore(t)
	stripe := &quotingRail{rail: newRail(processor.Stripe, true), rq: processor.RateQuote{ID: "fxq_r1", PerDollar: 95.2361, BasePerDollar: 94.2842}}
	reg := useQuotingRail(t, stripe)
	p := routing.Default()

	first, f := RequoteRenewal(ctx, reg, p, processor.Stripe, 800, "inr", 0, "alice", "sub_1")
	if f != nil || first.Quote.Amount != 76200 || first.Changed || first.Quote.QuoteID != "fxq_r1" {
		t.Fatalf("first: %+v %v", first, f)
	}
	stripe.rq.ID, stripe.rq.PerDollar = "fxq_r2", 96.5
	second, f := RequoteRenewal(ctx, reg, p, processor.Stripe, 800, "inr", first.Quote.Amount, "alice", "sub_1")
	if f != nil || second.Quote.Amount != 77200 || !second.Changed || second.Quote.UsdCents != 800 || second.Quote.QuoteID != "fxq_r2" {
		t.Fatalf("second: %+v %v", second, f)
	}
	if third, _ := RequoteRenewal(ctx, reg, p, processor.Stripe, 800, "inr", second.Quote.Amount, "alice", "sub_1"); third.Changed {
		t.Fatalf("an unchanged amount was announced: %+v", third)
	}
}

func TestAnIndianCardOrARupeePaymentIsNeverChargedOffSession(t *testing.T) {
	for _, c := range []struct {
		country, cur string
		needs        bool
	}{{"IN", "jpy", true}, {"in", "usd", true}, {"US", "inr", true}, {"IN", "inr", true}, {"US", "jpy", false}, {"", "eur", false}} {
		if got := RenewalNeedsBuyer(c.country, c.cur); got != c.needs {
			t.Errorf("card %q in %s needs the buyer = %v, want %v", c.country, c.cur, got, c.needs)
		}
	}
}

// chargingRail is a Stripe that quotes its own rate and takes the charge it is asked for.
type chargingRail struct {
	*quotingRail
	reqs []processor.PaymentRequest
}

func (r *chargingRail) Charge(_ context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	r.reqs = append(r.reqs, req)
	return &processor.PaymentResult{Success: true, ProcessorRef: "pi_renew_" + string(rune('a'+len(r.reqs)))}, nil
}

// localSub is a subscription sold on a card (the Square harness's sale), moved onto a
// Stripe card and billed in cur: the plan stays priced in dollars.
func localSub(t *testing.T, ctx ae.Context, name, cur, cardCountry string) (*organization.Organization, *datastore.Datastore, string, *chargingRail) {
	t.Helper()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg(name)
	db := datastore.New(org.Namespaced(ctx))
	m := squareMock("cust_"+name, "ccof_"+name, "sqpay_"+name)
	withFakeSquare(t, m)
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"go"}`, nil); r.StatusCode != 201 {
		t.Fatalf("sale: %d %s", r.StatusCode, bodyOf(r))
	}
	sub := parentSub(t, db, org.Name, "go")
	pm := paymentmethod.New(db)
	if err := pm.GetById(sub.DefaultPaymentMethod); err != nil {
		t.Fatal(err)
	}
	pm.ProviderType, pm.ProviderRef = string(processor.Stripe), "pm_card"
	pm.Card = &paymentmethod.CardDetails{Brand: "visa", Last4: "4242", Country: cardCountry}
	if err := pm.Update(); err != nil {
		t.Fatal(err)
	}
	s := reload(t, db, sub.Id())
	s.BillingCurrency = cur
	if err := s.Update(); err != nil {
		t.Fatal(err)
	}
	ch := &chargingRail{quotingRail: &quotingRail{rail: newRail(processor.Stripe, true), rq: processor.RateQuote{ID: "fxq_a", PerDollar: 150.4, BasePerDollar: 149.0, ExchangeRate: 1 / 150.4}}}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(ch)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
	return org, db, sub.Id(), ch
}

func billingEventsOf(t *testing.T, ctx context.Context, typ string) int {
	t.Helper()
	evs := make([]*billingevent.BillingEvent, 0)
	if _, err := billingevent.Query(webhookGuardDB(ctx)).Filter("Type=", typ).GetAll(&evs); err != nil {
		t.Fatal(err)
	}
	return len(evs)
}

// A renewal of a subscription billed in yen re-quotes, charges the new amount bound to
// the quote, says so when the amount moved, and credits the plan's dollars.
func TestARenewalOfALocalCurrencySubscriptionRequotesChargesAndBindsTheQuote(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db, id, ch := localSub(t, ctx, "local-renew", "jpy", "US")
	usd := lookupPlan("go").Price // dollars, in cents

	due(t, db, id)
	out := renewDue(ctx, org, "")
	if usd != 800 {
		t.Fatalf("the go plan is %d usd cents; this test is written for 800", usd)
	}
	if len(out) != 1 || !out[0].Success || len(ch.reqs) != 1 {
		t.Fatalf("first renewal: %+v, %d charges", out, len(ch.reqs))
	}
	r := ch.reqs[0]
	if string(r.Currency) != "jpy" || int64(r.Amount) != 1210 || r.Options["quoteId"] != "fxq_a" {
		t.Fatalf("charge = %d %s bound to %v; want 1210 yen ($8 at 150.4 is 1203.2, up to ten), bound to fxq_a", r.Amount, r.Currency, r.Options["quoteId"])
	}
	if out[0].AmountCents != int64(usd) {
		t.Fatalf("the invoice is settled for %d usd cents, want the plan's %d", out[0].AmountCents, usd)
	}
	if n := billingEventsOf(t, ctx, "renewal.amount_changed"); n != 0 {
		t.Fatalf("the first local charge changed nothing, but %d notices were emitted", n)
	}
	invs := make([]*billinginvoice.BillingInvoice, 0)
	billinginvoice.Query(db).Filter("SubscriptionId=", id).GetAll(&invs)
	bound := false
	for _, i := range invs {
		if l, ok := i.Metadata["local"].(map[string]interface{}); ok && l["quoteId"] == "fxq_a" && l["currency"] == "jpy" && int64(l["usdCents"].(float64)) == int64(usd) {
			bound = true
		}
	}
	if !bound {
		t.Fatalf("no invoice records the terms of the attempt: %+v", invs)
	}

	// The next period the yen is weaker: a fresh quote, a larger amount, a notice.
	ch.rq.ID, ch.rq.PerDollar, ch.rq.BasePerDollar = "fxq_b", 160.0, 158.0
	next := reload(t, db, id)
	next.PeriodEnd = time.Now().Add(-48 * time.Hour) // a period later than the first one's, not the same second
	next.PeriodStart = next.PeriodEnd.AddDate(0, -1, 0)
	if err := next.Update(); err != nil {
		t.Fatal(err)
	}
	out = renewDue(ctx, org, "")
	if len(out) != 1 || !out[0].Success || len(ch.reqs) != 2 {
		t.Fatalf("second renewal: %+v, %d charges", out, len(ch.reqs))
	}
	if ch.reqs[1].Amount <= ch.reqs[0].Amount || ch.reqs[1].Options["quoteId"] != "fxq_b" {
		t.Fatalf("second charge = %d bound to %v; want more than %d, bound to fxq_b", ch.reqs[1].Amount, ch.reqs[1].Options["quoteId"], ch.reqs[0].Amount)
	}
	if n := billingEventsOf(t, ctx, "renewal.amount_changed"); n != 1 {
		t.Fatalf("want one renewal.amount_changed, got %d", n)
	}
}

// An Indian card, or a payment in rupees, is not charged off-session: the buyer is sent
// a pay link, nothing is charged, and the subscription is past due, in its grace.
func TestARenewalThatNeedsTheBuyerEmitsActionRequiredAndChargesNothing(t *testing.T) {
	for _, c := range []struct{ name, cur, country string }{{"in-card", "jpy", "IN"}, {"inr-pay", "inr", "US"}} {
		t.Run(c.name, func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			org, db, id, ch := localSub(t, ctx, "renew-"+c.name, c.cur, c.country)
			due(t, db, id)
			out := renewDue(ctx, org, "")
			if len(out) != 1 || out[0].Success || len(ch.reqs) != 0 {
				t.Fatalf("renewal = %+v, %d charges; want none", out, len(ch.reqs))
			}
			if n := billingEventsOf(t, ctx, "renewal.action_required"); n != 1 {
				t.Fatalf("want one renewal.action_required, got %d", n)
			}
			if s := reload(t, db, id); s.Status != subscription.PastDue {
				t.Fatalf("status = %s, want past_due (the grace before suspension)", s.Status)
			}
		})
	}
}

func (r *chargingRail) CreateCustomer(context.Context, string, string, map[string]interface{}) (string, error) {
	return "cus_x", nil
}
func (r *chargingRail) AddPaymentMethod(context.Context, string, string) (string, error) {
	return "pm_card", nil
}
func (r *chargingRail) RemovePaymentMethod(context.Context, string, string) error { return nil }

// stripeBuyer is a buyer in country with a Stripe card on file, and a Stripe that quotes.
func stripeBuyer(t *testing.T, ctx ae.Context, name, country string, perDollar float64) (*organization.Organization, *datastore.Datastore, string, *chargingRail) {
	t.Helper()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg(name)
	db := datastore.New(org.Namespaced(ctx))
	ch := &chargingRail{quotingRail: &quotingRail{rail: newRail(processor.Stripe, true), rq: processor.RateQuote{ID: "fxq_s1", PerDollar: perDollar, BasePerDollar: perDollar, ExchangeRate: 1 / perDollar}}}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(ch)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
	pm := paymentmethod.New(db)
	pm.CustomerId, pm.UserId, pm.ProviderType, pm.ProviderRef = org.Name, org.Name, string(processor.Stripe), "pm_card"
	pm.Metadata = map[string]interface{}{"customerId": "cus_x"}
	pm.Card = &paymentmethod.CardDetails{Brand: "visa", Last4: "4242", Country: country}
	if err := pm.Create(); err != nil {
		t.Fatal(err)
	}
	return org, db, pm.Id(), ch
}

// An Indian buyer subscribes to Go, is quoted in rupees, is charged that amount on the
// quote the charge is bound to, and the subscription records INR; its renewal then
// needs the buyer and is announced, not charged.
func TestAnIndianBuyerIsSoldInRupeesAndTheRenewalNeedsThem(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db, pm, ch := stripeBuyer(t, ctx, "sale-in", "IN", 94.25)

	// The price the buyer is shown: the route's quote for the plan's dollars.
	pol, _ := ReadRouting(ctx, org.Name)
	shown, f := quoteVia(ctx, processorsForOrg(org), pol, processor.Stripe, 800, "IN", "", "", false)
	if f != nil || shown.Currency != "inr" || shown.Amount != 75400 {
		t.Fatalf("shown = %+v %v; want Rs 754", shown, f)
	}
	sale, err := Subscribe(ctx, org, SubscribeIn{MethodID: pm, PlanID: "go", Subject: org.Name, Country: "IN", QuotedAmount: shown.Amount, QuotedCurrency: shown.Currency})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if len(ch.reqs) != 1 || string(ch.reqs[0].Currency) != "inr" || int64(ch.reqs[0].Amount) != 75400 || ch.reqs[0].Options["quoteId"] != "fxq_s1" {
		t.Fatalf("charge = %+v; want Rs 754 bound to fxq_s1", ch.reqs)
	}
	if sale.AmountCents != 800 || sale.Currency != "usd" || sale.Quote == nil || sale.Quote.Currency != "inr" || sale.Quote.Amount != 75400 {
		t.Fatalf("sale = %+v quote %+v; want $8 credited, paid Rs 754", sale, sale.Quote)
	}
	if s := reload(t, db, sale.SubscriptionID); s.BillingCurrency != "inr" {
		t.Fatalf("billing currency = %q, want inr", s.BillingCurrency)
	}
	due(t, db, sale.SubscriptionID)
	out := renewDue(ctx, org, "")
	if len(out) != 1 || out[0].Success || len(ch.reqs) != 1 {
		t.Fatalf("renewal = %+v, %d charges; want none, the buyer is needed", out, len(ch.reqs))
	}
	if n := billingEventsOf(t, ctx, "renewal.action_required"); n != 1 {
		t.Fatalf("want one renewal.action_required, got %d", n)
	}
}

// A Korean buyer is sold in won; the renewal runs off-session at a fresh quote and says
// the amount moved.
func TestAKoreanBuyerIsSoldInWonAndRenewsOffSessionAtAFreshQuote(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db, pm, ch := stripeBuyer(t, ctx, "sale-kr", "KR", 1403.2)

	sale, err := Subscribe(ctx, org, SubscribeIn{MethodID: pm, PlanID: "go", Subject: org.Name, Country: "KR"})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if len(ch.reqs) != 1 || string(ch.reqs[0].Currency) != "krw" || int64(ch.reqs[0].Amount) != 11300 { // $8 x 1403.2 = 11225.6, up to a hundred won
		t.Fatalf("charge = %+v; want 11300 won", ch.reqs)
	}
	if s := reload(t, db, sale.SubscriptionID); s.BillingCurrency != "krw" {
		t.Fatalf("billing currency = %q, want krw", s.BillingCurrency)
	}
	ch.rq.ID, ch.rq.PerDollar, ch.rq.BasePerDollar = "fxq_s2", 1500, 1500
	next := reload(t, db, sale.SubscriptionID)
	next.PeriodEnd = time.Now().Add(-48 * time.Hour)
	next.PeriodStart = next.PeriodEnd.AddDate(0, -1, 0)
	if err := next.Update(); err != nil {
		t.Fatal(err)
	}
	out := renewDue(ctx, org, "")
	if len(out) != 1 || !out[0].Success || len(ch.reqs) != 2 {
		t.Fatalf("renewal = %+v, %d charges", out, len(ch.reqs))
	}
	if off, _ := ch.reqs[1].Options["offSession"].(bool); !off || int64(ch.reqs[1].Amount) != 12000 || ch.reqs[1].Options["quoteId"] != "fxq_s2" {
		t.Fatalf("renewal charge = %+v; want off-session, 12000 won, bound to fxq_s2", ch.reqs[1])
	}
	if n := billingEventsOf(t, ctx, "renewal.amount_changed"); n != 1 {
		t.Fatalf("want one renewal.amount_changed, got %d", n)
	}
}

// A buyer shown a price the locked quote no longer matches is refused before any charge.
func TestASaleIsRefusedWhenTheLockedQuoteIsNotThePriceTheBuyerSaw(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, _, pm, ch := stripeBuyer(t, ctx, "sale-stale", "KR", 1403.2)
	_, err := Subscribe(ctx, org, SubscribeIn{MethodID: pm, PlanID: "go", Subject: org.Name, Country: "KR", QuotedAmount: 11000, QuotedCurrency: "krw"})
	if err == nil || len(ch.reqs) != 0 {
		t.Fatalf("a stale price was charged: %v, %d charges", err, len(ch.reqs))
	}
}

// authRail asks for the bank's step on the first charge of a card, and takes the charge
// when the buyer comes back naming the intent they authenticated.
type authRail struct {
	*chargingRail
	failFinish bool // the bank's step was never completed, or its rate lock lapsed
}

func (r *authRail) Charge(_ context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	r.reqs = append(r.reqs, req)
	if strings.HasPrefix(req.Token, "pi_") {
		if r.failFinish {
			return &processor.PaymentResult{ErrorMessage: "payment_intent_fx_quote_invalid"}, errors.New("payment_intent_fx_quote_invalid")
		}
		return &processor.PaymentResult{Success: true, ProcessorRef: "ch_after_3ds"}, nil
	}
	return &processor.PaymentResult{Error: &processor.ActionRequired{ID: "pi_3ds", ClientSecret: "pi_3ds_secret_x"}}, &processor.ActionRequired{ID: "pi_3ds", ClientSecret: "pi_3ds_secret_x"}
}

// An Indian card needs its bank on the first charge. The sale answers the step and
// records the attempt bound to its quote; the buyer comes back with the intent they
// authenticated and is charged exactly what was quoted, once: one subscription, in INR,
// the quote on its payment, and the same intent cannot buy a second.
func TestAnIndianSaleFinishedAfterTheBanksStepIsOneCreditInRupeesOnItsQuote(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db, pm, ch := stripeBuyer(t, ctx, "sale-in-3ds", "IN", 94.25)
	rail := &authRail{chargingRail: ch}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(rail)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })

	in := SubscribeIn{MethodID: pm, PlanID: "go", Subject: org.Name, Country: "IN"}
	_, err := Subscribe(ctx, org, in)
	var ar authRequired
	if err == nil || json.Unmarshal([]byte(err.Error()), &ar) != nil || ar.ID != "pi_3ds" {
		t.Fatalf("first charge = %v; want the bank's step", err)
	}
	rec, found, _ := pendingintent.Get("pi_3ds")
	if !found || rec.Cents != 800 || rec.PaidCurrency != "inr" || rec.PaidCents != 75400 || rec.QuoteID != "fxq_s1" {
		t.Fatalf("attempt = %+v; want 800 usd cents paid Rs 754 on fxq_s1", rec)
	}
	if subs, _ := userSubscriptions(db, org.Name, false); len(subs) != 0 {
		t.Fatalf("%d subscriptions before the buyer is back", len(subs))
	}

	in.SourceID = "pi_3ds"
	sale, err := Subscribe(ctx, org, in)
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	last := rail.reqs[len(rail.reqs)-1]
	if last.Token != "pi_3ds" || string(last.Currency) != "inr" || int64(last.Amount) != 75400 {
		t.Fatalf("finishing charge = %+v; want the intent for Rs 754", last)
	}
	if sale.AmountCents != 800 || sale.Quote == nil || sale.Quote.Currency != "inr" || sale.Quote.Amount != 75400 || sale.Quote.QuoteID != "fxq_s1" {
		t.Fatalf("sale = %+v quote %+v", sale, sale.Quote)
	}
	if s := reload(t, db, sale.SubscriptionID); s.BillingCurrency != "inr" {
		t.Fatalf("billing currency = %q, want inr", s.BillingCurrency)
	}
	if po, ok, _ := paymentorg.Get("ch_after_3ds"); !ok || po.PaidCurrency != "inr" || po.PaidCents != 75400 || po.CreditCents != 800 {
		t.Fatalf("payment record = %+v; want paid Rs 754 for 800 usd cents", po)
	}
	// The same intent is spent: a second finish buys nothing.
	calls := len(rail.reqs)
	if _, err := Subscribe(ctx, org, in); err == nil && len(rail.reqs) != calls {
		t.Fatalf("the spent intent was charged again")
	}
	if subs, _ := userSubscriptions(db, org.Name, false); len(subs) != 1 {
		t.Fatalf("%d subscriptions after finishing twice; want 1", len(subs))
	}
}

// A buyer who comes back after the rate lock lapsed, and whose payment did not go through,
// is shown the new price and charged nothing; coming back with it confirmed, they are sold
// at the fresh quote.
func TestABuyerReturningAfterTheLockLapsedIsReQuotedAndShownTheNewPrice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db, pm, ch := stripeBuyer(t, ctx, "sale-lapsed", "IN", 94.25)
	rail := &authRail{chargingRail: ch}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(rail)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })

	in := SubscribeIn{MethodID: pm, PlanID: "go", Subject: org.Name, Country: "IN", QuotedAmount: 75400, QuotedCurrency: "inr"}
	if _, err := Subscribe(ctx, org, in); err == nil {
		t.Fatal("the first charge should ask for the bank's step")
	}
	// An hour on: the lock has lapsed and the rupee has moved.
	rec, _, _ := pendingintent.Get("pi_3ds")
	rec.LockExpiresAt = time.Now().Add(-time.Minute).Unix()
	if err := rec.Put(); err != nil {
		t.Fatal(err)
	}
	ch.rq.ID, ch.rq.PerDollar, ch.rq.BasePerDollar = "fxq_s9", 96.0, 96.0
	rail.failFinish = true
	in.SourceID = "pi_3ds"
	_, err := Subscribe(ctx, org, in)
	if err == nil || !strings.Contains(err.Error(), "lapsed") || !strings.Contains(err.Error(), "76800 inr") {
		t.Fatalf("returning after the lapse = %v; want the new price, Rs 768, named", err)
	}
	if subs, _ := userSubscriptions(db, org.Name, false); len(subs) != 0 {
		t.Fatalf("%d subscriptions opened by a payment that did not go through", len(subs))
	}
	// Seeing the new price, the buyer confirms it and is sold at the fresh quote.
	rail.failFinish = false
	fresh := SubscribeIn{MethodID: pm, PlanID: "go", Subject: org.Name, Country: "IN", QuotedAmount: 76800, QuotedCurrency: "inr"}
	if _, err := Subscribe(ctx, org, fresh); err == nil {
		t.Fatal("the fresh quote's first charge should ask for the bank's step again")
	}
}
