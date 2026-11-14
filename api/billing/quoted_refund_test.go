package billing

import (
	"context"
	"net/url"
	"strconv"
	"testing"

	"github.com/hanzoai/commerce/payment/fx"
)

func TestAQuotedRefundIsTakenBackInTheDollarsItCredited(t *testing.T) {
	for _, c := range []struct {
		credit, refunded, paid, want int64
		why                          string
	}{
		{500, 462, 462, 500, "a full refund is the whole credit exactly"},
		{500, 231, 462, 250, "half the payment is half the credit"},
		{500, 100, 462, 108, "a part is its proportional share, rounded to the nearest cent"},
		{500, 9999, 462, 500, "a refund can never take more than was credited"},
		{500, 0, 462, 0, "nothing refunded takes nothing"},
		{500, 100, 0, 0, "a payment with no recorded charge takes nothing by this rule"},
	} {
		if got := quotedShare(c.credit, c.refunded, c.paid); got != c.want {
			t.Errorf("quotedShare(%d, %d, %d) = %d, want %d: %s", c.credit, c.refunded, c.paid, got, c.want, c.why)
		}
	}
}

// A payment made in euros against a quote of 500 usd is refunded in euros: the wallet
// gives back the 500 dollars it was credited, never the euro figure read as dollars.
func TestLiveStripe_AQuotedPaymentIsRefundedForTheDollarsItCredited(t *testing.T) {
	l := liveSetup(t)
	old := fx.Rate
	fx.Rate = func(context.Context, string) (float64, error) { return 0.9, nil }
	t.Cleanup(func() { fx.Rate = old })

	pay := func(key string) (intent, charge string, paid int64) {
		out, f := OpenTopupIntent(*l.ctx, l.org, TopupIntentIn{Subject: l.subject, Country: "DE", AmountCents: 500, IdempotencyKey: key})
		if f != nil {
			t.Fatalf("open: %d %s (%v)", f.Status, f.Message, f.Err)
		}
		c := l.call("POST", "/payment_intents/"+out.IntentID+"/confirm", url.Values{"payment_method": {"pm_card_visa"}, "return_url": {"https://pay.hanzo.ai/done"}})
		if str(c["status"]) != "succeeded" {
			t.Fatalf("confirm = %v", c["status"])
		}
		ev := l.event("payment_intent.succeeded", func(o map[string]any) bool { return str(o["id"]) == out.IntentID })
		if status, resp := l.deliver(ev, l.hook); status != 200 {
			t.Fatalf("settlement delivery = %d %v", status, resp)
		}
		return out.IntentID, str(c["latest_charge"]), out.Amount
	}
	refund := func(charge string, cents int64) {
		r := l.call("POST", "/refunds", url.Values{"charge": {charge}, "amount": {strconv.FormatInt(cents, 10)}})
		ev := l.event("refund.created", func(o map[string]any) bool { return str(o["id"]) == str(r["id"]) && str(o["status"]) == "succeeded" })
		if status, resp := l.deliver(ev, l.hook); status != 200 {
			t.Fatalf("refund delivery = %d %v", status, resp)
		}
		l.deliver(ev, l.hook) // the replay
	}

	_, chargeA, paidA := pay("quoted-refund-a")
	if got := l.ledger.credits(); len(got) != 1 || got[0].AmountCents != 500 || got[0].Currency != "usd" {
		t.Fatalf("credits = %+v; want one of 500 usd", got)
	}
	refund(chargeA, paidA)
	if len(l.ledger.clawbacks) != 1 || l.ledger.clawbacks[0].AmountCents != 500 {
		t.Fatalf("a full euro refund took back %+v; want one clawback of exactly 500", l.ledger.clawbacks)
	}

	// Two halves take back the credit between them and no more, each replay once.
	_, chargeB, paidB := pay("quoted-refund-b")
	refund(chargeB, paidB/2)
	refund(chargeB, paidB-paidB/2)
	var b int64
	for _, c := range l.ledger.clawbacks[1:] {
		b += c.AmountCents
	}
	if len(l.ledger.clawbacks) != 3 || b != 500 {
		t.Fatalf("two half refunds took back %+v; want two clawbacks summing to 500", l.ledger.clawbacks[1:])
	}
}

// A second top-up of the same amount, opened while the first sits unpaid, is the first
// (a double click); opened after the first was paid, it is a new payment that can be paid.
func TestLiveStripe_AnIntentIsReplayedOnlyWhileItCanStillBePaid(t *testing.T) {
	l := liveSetup(t)
	old := fx.Rate
	fx.Rate = func(context.Context, string) (float64, error) { return 0.9, nil }
	t.Cleanup(func() { fx.Rate = old })

	open := func() *TopupIntentOut {
		out, f := OpenTopupIntent(*l.ctx, l.org, TopupIntentIn{Subject: l.subject, Country: "DE", AmountCents: 500})
		if f != nil {
			t.Fatalf("open: %d %s (%v)", f.Status, f.Message, f.Err)
		}
		return out
	}
	first := open()
	again := open()
	if again.IntentID != first.IntentID || again.ClientSecret != first.ClientSecret {
		t.Fatalf("a double click opened %s then %s; want the one unpaid intent", first.IntentID, again.IntentID)
	}
	c := l.call("POST", "/payment_intents/"+first.IntentID+"/confirm", url.Values{"payment_method": {"pm_card_visa"}, "return_url": {"https://pay.hanzo.ai/done"}})
	if str(c["status"]) != "succeeded" {
		t.Fatalf("confirm = %v", c["status"])
	}
	next := open()
	if next.IntentID == first.IntentID {
		t.Fatal("a repeat after the first was paid replayed the finished intent; the browser cannot pay it")
	}
	pi := l.call("GET", "/payment_intents/"+next.IntentID, nil)
	if str(pi["status"]) != "requires_payment_method" {
		t.Fatalf("the new intent is %v; want one that can be paid", pi["status"])
	}
	l.call("POST", "/payment_intents/"+next.IntentID+"/cancel", nil)
}
