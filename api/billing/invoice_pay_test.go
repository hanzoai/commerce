package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// payRail is a Stripe that quotes its own rate and answers a charge as the test says:
// success, the bank's step, or a decline. A cancel is recorded.
type payRail struct {
	*chargingRail
	mode      string // "pay", "auth" or "decline"
	finished  bool   // the buyer has authenticated the intent at their bank
	cancelled []string
	intents   int
}

func (r *payRail) Charge(_ context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	r.reqs = append(r.reqs, req)
	if len(req.Token) > 3 && req.Token[:3] == "pi_" {
		return &processor.PaymentResult{Success: true, ProcessorRef: "ch_" + req.Token}, nil
	}
	switch r.mode {
	case "auth":
		r.intents++
		id := "pi_inv_" + string(rune('a'+r.intents))
		ar := &processor.ActionRequired{ID: id, ClientSecret: id + "_secret_x"}
		return &processor.PaymentResult{Error: ar}, ar
	case "decline":
		return &processor.PaymentResult{ErrorMessage: "your card was declined"}, errors.New("your card was declined")
	}
	return &processor.PaymentResult{Success: true, ProcessorRef: "ch_inv_" + string(rune('a'+len(r.reqs)))}, nil
}

func (r *payRail) GetTransaction(_ context.Context, id string) (*processor.Transaction, error) {
	if r.finished {
		return &processor.Transaction{ID: id, Status: "succeeded", ProcessorRef: "ch_" + id}, nil
	}
	return &processor.Transaction{ID: id, Status: "requires_action", Metadata: map[string]interface{}{"client_secret": id + "_secret_x"}}, nil
}

func (r *payRail) CancelIntent(_ context.Context, id string) error {
	r.cancelled = append(r.cancelled, id)
	return nil
}

// openInvoice is a Korean subscriber's renewal invoice left open (the buyer was not asked
// at renewal), and the rail that will answer paying it.
func openInvoice(t *testing.T, ctx ae.Context, name string) (*organization.Organization, *datastore.Datastore, *billinginvoice.BillingInvoice, *payRail) {
	t.Helper()
	org, db, pm, ch := stripeBuyer(t, ctx, name, "KR", 1403.2)
	rail := &payRail{chargingRail: ch, mode: "pay"}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(rail)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
	sale, err := Subscribe(ctx, org, SubscribeIn{MethodID: pm, PlanID: "go", Subject: org.Name, Country: "KR"})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	// The next renewal declines, so its invoice stays open.
	rail.mode = "decline"
	next := reload(t, db, sale.SubscriptionID)
	next.PeriodEnd = time.Now().Add(-48 * time.Hour)
	next.PeriodStart = next.PeriodEnd.AddDate(0, -1, 0)
	if err := next.Update(); err != nil {
		t.Fatal(err)
	}
	if out := renewDue(ctx, org, ""); len(out) != 1 || out[0].Success {
		t.Fatalf("renewal = %+v; want a decline that leaves the invoice open", out)
	}
	invs := make([]*billinginvoice.BillingInvoice, 0)
	billinginvoice.Query(db).Filter("SubscriptionId=", sale.SubscriptionID).Filter("Status=", billinginvoice.Open).GetAll(&invs)
	if len(invs) != 1 {
		t.Fatalf("%d open invoices; want 1", len(invs))
	}
	rail.reqs = nil
	return org, db, invs[0], rail
}

func TestAnOpenInvoiceReadsWithItsPriceInTheBuyersCurrency(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db, inv, _ := openInvoice(t, ctx, "inv-read")
	q := invoiceQuote(ctx, org, db, inv)
	if q == nil || q.Currency != "krw" || q.UsdCents != 800 || q.Amount != 11300 || q.Disclosure == "" || q.QuoteID != "" {
		t.Fatalf("open invoice quote = %+v; want 800 usd cents shown as 11300 won, unlocked", q)
	}
}

func TestPayingAnInvoiceAnswersPaidAndPayingItAgainChargesNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, _, inv, rail := openInvoice(t, ctx, "inv-paid")
	rail.mode = "pay"
	got, f := PayInvoiceNow(ctx, org, inv.Id(), nil)
	if f != nil || got.Status != PayPaid || got.Invoice.Status != "paid" || len(rail.reqs) != 1 {
		t.Fatalf("pay = %+v %v, %d charges; want paid, charged once", got, f, len(rail.reqs))
	}
	if int64(rail.reqs[0].Amount) != 11300 || string(rail.reqs[0].Currency) != "krw" || rail.reqs[0].Options["quoteId"] == "" {
		t.Fatalf("charge = %+v; want 11300 won bound to a quote", rail.reqs[0])
	}
	if q := got.Invoice.Quote; q == nil || q.Amount != 11300 || q.QuoteID == "" || q.Currency != "krw" {
		t.Fatalf("the paid invoice carries quote %+v; want the bound one", q)
	}
	again, f := PayInvoiceNow(ctx, org, inv.Id(), nil)
	if f != nil || again.Status != PayPaid || len(rail.reqs) != 1 {
		t.Fatalf("second pay = %+v %v, %d charges; want paid and no second charge", again, f, len(rail.reqs))
	}
}

func TestPayingAnInvoiceAnswersDeclinedWithTheReason(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, _, inv, rail := openInvoice(t, ctx, "inv-declined")
	rail.mode = "decline"
	got, f := PayInvoiceNow(ctx, org, inv.Id(), nil)
	if f != nil || got.Status != PayDeclined || got.Reason == "" || got.Invoice.Status == "paid" || got.Authentication != nil {
		t.Fatalf("pay = %+v %v; want declined with a reason", got, f)
	}
}

func TestPayingAnInvoiceThatNeedsTheBankAnswersAuthenticationWithTheQuoteAndAReQuoteAfterTheLockLapses(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, _, inv, rail := openInvoice(t, ctx, "inv-auth")
	rail.mode = "auth"
	got, f := PayInvoiceNow(ctx, org, inv.Id(), nil)
	if f != nil || got.Status != PayAuthentication || got.Authentication == nil {
		t.Fatalf("pay = %+v %v; want the bank's step", got, f)
	}
	a := got.Authentication
	if a.Code != "authentication_required" || a.ID != "pi_inv_b" || a.ClientSecret == "" || a.Quote == nil || a.Quote.Currency != "krw" || a.Quote.Amount != 11300 || a.Quote.QuoteID == "" {
		t.Fatalf("step = %+v quote %+v; want pi_inv_b, its secret and the quote it is bound to", a, a.Quote)
	}
	first := a.Quote.QuoteID
	// Pressing pay again before the lock lapses: the same intent, the same quote, no new charge.
	n := len(rail.reqs)
	if again, _ := PayInvoiceNow(ctx, org, inv.Id(), nil); again.Status != PayAuthentication || again.Authentication.Quote.QuoteID != first {
		t.Fatalf("a second press = %+v; want the same step on the same quote", again)
	}
	_ = n
	// The lock lapses and the rate moves: the old intent is cancelled and the buyer is
	// shown a new amount on a new quote before they authorize anything.
	rec, _, _ := pendingintent.Get(a.ID)
	rec.LockExpiresAt = time.Now().Add(-time.Minute).Unix()
	if err := rec.Put(); err != nil {
		t.Fatal(err)
	}
	rail.rq.ID, rail.rq.PerDollar, rail.rq.BasePerDollar = "fxq_new", 1500, 1500
	re, f := PayInvoiceNow(ctx, org, inv.Id(), nil)
	if f != nil || re.Status != PayAuthentication || re.Authentication.ID == a.ID {
		t.Fatalf("after the lapse = %+v %v; want a fresh step", re, f)
	}
	if q := re.Authentication.Quote; q == nil || q.QuoteID != "fxq_new" || q.Amount != 12000 {
		t.Fatalf("re-quote = %+v; want 12000 won on fxq_new", q)
	}
	if len(rail.cancelled) != 1 || rail.cancelled[0] != a.ID {
		t.Fatalf("cancelled %v; want the lapsed intent %s", rail.cancelled, a.ID)
	}
}

// The buyer authenticated at their bank and presses pay again: the invoice settles from the
// intent they authenticated, at the amount they saw, and the quote stays on it.
func TestAnInvoiceIsSettledFromTheIntentTheBuyerAuthenticated(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, _, inv, rail := openInvoice(t, ctx, "inv-finish")
	rail.mode = "auth"
	step, f := PayInvoiceNow(ctx, org, inv.Id(), nil)
	if f != nil || step.Status != PayAuthentication {
		t.Fatalf("pay = %+v %v", step, f)
	}
	rail.finished = true
	got, f := PayInvoiceNow(ctx, org, inv.Id(), nil)
	if f != nil || got.Status != PayPaid {
		t.Fatalf("after authentication = %+v %v; want paid", got, f)
	}
	last := rail.reqs[len(rail.reqs)-1]
	if last.Token != step.Authentication.ID || int64(last.Amount) != 11300 || string(last.Currency) != "krw" {
		t.Fatalf("finishing charge = %+v; want the authenticated intent for 11300 won", last)
	}
}
