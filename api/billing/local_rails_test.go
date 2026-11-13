package billing

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// stripeDouble answers what the settlement asks Stripe about a charge, and what a
// cancel does.
type stripeDouble struct {
	processor.PaymentProcessor
	tx        *processor.Transaction
	cancelled []string
}

func (*stripeDouble) Type() processor.ProcessorType        { return processor.Stripe }
func (*stripeDouble) IsAvailable(context.Context) bool     { return true }
func (*stripeDouble) SupportedCurrencies() []currency.Type { return nil }
func (*stripeDouble) ChargeSound(context.Context, string, int64) (bool, error) {
	return true, nil
}
func (d *stripeDouble) GetTransaction(context.Context, string) (*processor.Transaction, error) {
	return d.tx, nil
}
func (d *stripeDouble) CancelIntent(_ context.Context, id string) error {
	d.cancelled = append(d.cancelled, id)
	return nil
}

func useStripe(t *testing.T, d *stripeDouble) {
	t.Helper()
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(d)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
}

// A Brazilian buyer is quoted R$27.50 for $5.00 of credit and pays by Pix.
func quotedPix(ctx context.Context, t *testing.T, id string) (*organization.Organization, *fakeLedger) {
	t.Helper()
	org := routingOrg(ctx, t)
	led := newFakeLedger()
	injectLedger(t, led)
	if err := pendingintent.PutQuoted(id, org.Name, "buyer", pendingintent.Topup, 500, "usd", 2750, "brl", false); err != nil {
		t.Fatal(err)
	}
	return org, led
}

func succeeded(id, charge string, cents int64, cur string) *processor.WebhookEvent {
	return &processor.WebhookEvent{
		ID: "evt_" + id, Type: "payment.completed", Processor: processor.Stripe,
		Data: map[string]interface{}{"id": charge, "status": "COMPLETED", "payment_intent": id, "amount": float64(cents), "currency": cur},
	}
}

func TestAPixPaymentCreditsTheQuotedDollarsExactlyOnce(t *testing.T) {
	ctx := heldStore(t)
	org, led := quotedPix(ctx, t, "pi_pix_once")
	useStripe(t, &stripeDouble{tx: &processor.Transaction{ID: "pi_pix_once", ProcessorRef: "ch_pix_once", Amount: 2750, Currency: "brl", Status: "succeeded"}})
	db := datastore.New(org.Namespaced(ctx))

	// The webhook, delivered three times, and the reconciler after it.
	for i := 0; i < 3; i++ {
		if err := applySettlementEvent(ctx, db, org, succeeded("pi_pix_once", "ch_pix_once", 2750, "brl")); err != nil {
			t.Fatalf("delivery %d: %v", i, err)
		}
	}
	p, _, _ := pendingintent.Get("pi_pix_once")
	if err := reconcileIntent(ctx, p); err != nil {
		t.Fatalf("reconciler: %v", err)
	}

	got := led.credits()
	if len(got) != 1 {
		t.Fatalf("want exactly one credit, got %d: %+v", len(got), got)
	}
	if got[0].AmountCents != 500 || got[0].Currency != "usd" || got[0].IdempotencyKey != "ch_pix_once" || got[0].Subject != "buyer" || !got[0].Cash {
		t.Fatalf("the wallet must get the $5.00 the buyer bought, keyed on the charge: %+v", got[0])
	}
}

func TestAPaymentThatSettlesForAnotherAmountCreditsNothing(t *testing.T) {
	ctx := heldStore(t)
	org, led := quotedPix(ctx, t, "pi_pix_short")
	db := datastore.New(org.Namespaced(ctx))
	useStripe(t, &stripeDouble{tx: &processor.Transaction{ID: "pi_pix_short", Amount: 2750, Currency: "brl", Status: "succeeded"}})
	for _, ev := range []*processor.WebhookEvent{
		succeeded("pi_pix_short", "ch_a", 500, "brl"),  // the dollar figure in reais
		succeeded("pi_pix_short", "ch_b", 2750, "usd"), // the right figure, the wrong money
		succeeded("pi_pix_short", "ch_c", 2000, "brl"),
	} {
		_ = applySettlementEvent(ctx, db, org, ev)
	}
	if n := len(led.credits()); n != 0 {
		t.Fatalf("a payment that is not the quote must credit nothing, credited %d", n)
	}
}

func TestPendingFailedAndExpiredPaymentsCreditNothing(t *testing.T) {
	ctx := heldStore(t)
	org, led := quotedPix(ctx, t, "pi_pix_never")
	_ = org
	// What Stripe says while a Pix code or a boleto is open, after an attempt is
	// refused, and when the voucher lapses: none of it is a settlement.
	for _, stripeType := range []string{"payment.pending", "payment.failed", "payment.canceled"} {
		evt := &processor.WebhookEvent{ID: "evt_" + stripeType, Type: stripeType, Processor: processor.Stripe,
			Data: map[string]interface{}{"id": "pi_pix_never", "status": "processing", "amount": float64(2750), "currency": "brl", "latest_charge": "ch_never"}}
		normalizeStripeWebhookEvent(evt)
		if evt.Type != stripeType {
			t.Fatalf("%s was dropped instead of recorded: %q", stripeType, evt.Type)
		}
		if isSettlementEvent(evt.Type) {
			t.Fatalf("%s would be treated as a settlement", stripeType)
		}
	}
	// A "succeeded" that is not (a payment still processing) normalizes to nothing.
	evt := &processor.WebhookEvent{ID: "evt_p", Type: "payment.completed", Processor: processor.Stripe,
		Data: map[string]interface{}{"id": "pi_pix_never", "status": "processing", "latest_charge": "ch_never", "amount": float64(2750), "currency": "brl"}}
	normalizeStripeWebhookEvent(evt)
	if evt.Type != "" {
		t.Fatalf("a payment still processing was passed on as %q", evt.Type)
	}

	// The reconciler cancels a voucher that outlived its life and closes the record.
	d := &stripeDouble{tx: &processor.Transaction{ID: "pi_pix_never", Amount: 2750, Currency: "brl", Status: "requires_payment_method"}}
	useStripe(t, d)
	p, _, _ := pendingintent.Get("pi_pix_never")
	p.CreatedAt = p.CreatedAt.Add(-delayedAbandonAfter - 1)
	if err := reconcileIntent(ctx, p); err != nil {
		t.Fatal(err)
	}
	if len(d.cancelled) != 1 || d.cancelled[0] != "pi_pix_never" {
		t.Fatalf("an expired voucher must be cancelled at Stripe, got %v", d.cancelled)
	}
	if closed, _, _ := pendingintent.Get("pi_pix_never"); !closed.Done {
		t.Fatal("the expired payment's record stays open")
	}
	if n := len(led.credits()); n != 0 {
		t.Fatalf("credited %d times for payments that never settled", n)
	}
}

func TestAVoucherIsNotCancelledWhileItCanStillBePaid(t *testing.T) {
	ctx := heldStore(t)
	quotedPix(ctx, t, "pi_boleto_open")
	d := &stripeDouble{tx: &processor.Transaction{ID: "pi_boleto_open", Amount: 2750, Currency: "brl", Status: "processing"}}
	useStripe(t, d)
	p, _, _ := pendingintent.Get("pi_boleto_open")
	p.CreatedAt = p.CreatedAt.Add(-3 * 24 * 3600 * 1e9) // three days old: past a card's 48 hours
	if err := reconcileIntent(ctx, p); err != nil {
		t.Fatal(err)
	}
	if len(d.cancelled) != 0 {
		t.Fatal("a boleto still inside its life was cancelled")
	}
}
