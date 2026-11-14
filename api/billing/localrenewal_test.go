package billing

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
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
