package billing

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// flaky is a ledger that refuses its first n credits, as one that is down does.
type flaky struct {
	*fakeLedger
	down atomic.Int32
}

func (f *flaky) Credit(ctx context.Context, in creditledger.CreditInput) (string, int64, error) {
	if f.down.Add(-1) >= 0 {
		return "", 0, errors.New("the ledger is down")
	}
	return f.fakeLedger.Credit(ctx, in)
}

// card is a processor whose charges clear, counting them.
type clearing struct {
	processor.PaymentProcessor
	charges atomic.Int32
}

func (*clearing) Type() processor.ProcessorType        { return processor.Square }
func (*clearing) IsAvailable(context.Context) bool     { return true }
func (*clearing) SupportedCurrencies() []currency.Type { return []currency.Type{currency.USD} }
func (c *clearing) Charge(_ context.Context, r processor.PaymentRequest) (*processor.PaymentResult, error) {
	n := c.charges.Add(1)
	return &processor.PaymentResult{Success: true, ProcessorRef: "sq_pay_" + string(rune('0'+n)), Status: "COMPLETED"}, nil
}

func useCard(t *testing.T, c *clearing) {
	t.Helper()
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.SetConfig(&processor.RegistryConfig{ProcessorPriority: []processor.ProcessorType{processor.Square}, DisabledProcessors: map[processor.ProcessorType]bool{}})
		reg.Register(c)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
}

func take(ctx context.Context, org *organization.Organization, key string) (*TakePaymentOut, *PaymentFault) {
	return TakePayment(ctx, org, TakePaymentIn{
		SourceID: "cnon:card-ok", AmountCents: 500, Currency: "usd", Subject: "buyer",
		Ledger: "customerco", IdempotencyKey: key, Provider: "square",
	})
}

// Commerce posts the spendable credit itself, at the ledger the host named, once, keyed
// on the processor's reference — and a retry, a replay and the provider's own webhook
// for the same payment add nothing.
func TestTheCardCoreIsTheOnlyWriterOfTheCredit(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	led := newFakeLedger()
	injectLedger(t, led)
	c := &clearing{}
	useCard(t, c)

	first, f := take(ctx, org, "k-once")
	if f != nil {
		t.Fatalf("take: %d %s (%v)", f.Status, f.Message, f.Err)
	}
	again, f := take(ctx, org, "k-once")
	if f != nil || again.TransactionID != first.TransactionID {
		t.Fatalf("the replay = %+v %v", again, f)
	}
	// The provider's webhook for the same payment, delivered twice.
	db := datastore.New(org.Namespaced(ctx))
	ev := &processor.WebhookEvent{ID: "evt_sq", Type: "payment.completed", Processor: processor.Square, Data: map[string]interface{}{
		"payment": map[string]interface{}{"id": first.ProcessorRef, "status": "COMPLETED",
			"amount_money": map[string]interface{}{"amount": float64(500), "currency": "USD"}}}}
	for i := 0; i < 2; i++ {
		if err := applySettlementEvent(ctx, db, org, ev); err != nil {
			t.Fatal(err)
		}
	}

	got := led.credits()
	if len(got) != 1 || c.charges.Load() != 1 {
		t.Fatalf("%d credits and %d charges, want one of each: %+v", len(got), c.charges.Load(), got)
	}
	if g := got[0]; g.Org != "customerco" || g.Subject != "buyer" || g.AmountCents != 500 || !g.Cash || g.IdempotencyKey != first.ProcessorRef {
		t.Fatalf("the credit = %+v", g)
	}
}

// A ledger that is down leaves the payment taken and recoverable: the caller is told
// to repeat the request, the repeat posts the credit, and the card is charged once.
func TestALedgerOutageLeavesThePaymentRecoverable(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	led := &flaky{fakeLedger: newFakeLedger()}
	led.down.Store(2)
	injectLedger(t, led)
	c := &clearing{}
	useCard(t, c)

	for i := 0; i < 2; i++ {
		if _, f := take(ctx, org, "k-down"); f == nil || f.Status != 503 {
			t.Fatalf("attempt %d with the ledger down: %+v, want 503", i, f)
		}
	}
	out, f := take(ctx, org, "k-down")
	if f != nil {
		t.Fatalf("the repeat: %d %s", f.Status, f.Message)
	}
	if got := led.credits(); len(got) != 1 || got[0].IdempotencyKey != out.ProcessorRef || c.charges.Load() != 1 {
		t.Fatalf("%d credits, %d charges after recovery, want one each: %+v", len(got), c.charges.Load(), got)
	}
}

// The saved-card top-up posts through the same writer, so it is credited once on a retry.
func TestTheSavedCardTopupPostsThroughTheSameWriter(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	led := newFakeLedger()
	injectLedger(t, led)
	r := receipt{TransactionId: "t1", ProcessorRef: "sq_saved_1", AmountCents: 700}
	for i := 0; i < 3; i++ {
		if err := bankAt(ctx, org, "customerco", "buyer", currency.USD, r, "Top-up via card"); err != nil {
			t.Fatal(err)
		}
	}
	if got := led.credits(); len(got) != 1 || got[0].Org != "customerco" || got[0].AmountCents != 700 {
		t.Fatalf("credits = %+v", got)
	}
}
