package billing

import (
	"testing"

	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
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
	// The next period, the rupee has moved: a fresh quote, the credit still $8, and an event.
	stripe.rq.ID, stripe.rq.PerDollar = "fxq_r2", 96.5
	second, f := RequoteRenewal(ctx, reg, p, processor.Stripe, 800, "inr", first.Quote.Amount, "alice", "sub_1")
	if f != nil || second.Quote.Amount != 77200 || !second.Changed || second.Quote.UsdCents != 800 || second.Quote.QuoteID != "fxq_r2" {
		t.Fatalf("second: %+v %v", second, f)
	}
	evs := make([]*billingevent.BillingEvent, 0)
	if _, err := billingevent.Query(webhookGuardDB(ctx)).Filter("Type=", "renewal.amount_changed").GetAll(&evs); err != nil || len(evs) != 1 {
		t.Fatalf("want one renewal.amount_changed, got %d (%v)", len(evs), err)
	}
	// The same rate again moves nothing and announces nothing.
	third, _ := RequoteRenewal(ctx, reg, p, processor.Stripe, 800, "inr", second.Quote.Amount, "alice", "sub_1")
	if third.Changed {
		t.Fatalf("an unchanged amount was announced: %+v", third)
	}
}

func TestIndiaRenewalsOver15000NeedTheBuyerAndMandatesCarryHeadroom(t *testing.T) {
	for _, c := range []struct {
		cur   string
		minor int64
		off   bool
	}{{"inr", 1500000, true}, {"inr", 1500100, false}, {"inr", 1870000, false}, {"inr", 76200, true}, {"krw", 99999999, true}, {"eur", 99999999, true}} {
		if got := RenewalMode(c.cur, c.minor); got != c.off {
			t.Errorf("%s %d off-session = %v, want %v", c.cur, c.minor, got, c.off)
		}
	}
	// Ultra at about Rs 18.7k: the mandate maximum is the amount plus 10%, to the rupee.
	if got := MandateMax(1870000, 100); got != 2057000 {
		t.Errorf("MandateMax = %d, want 2057000", got)
	}
}
