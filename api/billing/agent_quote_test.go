package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A sale that carries the price the buyer was quoted is made at that price or not at all:
// it is refused before the idempotency guard and before the card is touched.
func TestSubscribe_AQuotedChargeIsBoundBeforeTheCardIsTouched(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatal(err)
	}
	org := moneyOrg("quote-bound")
	m := squareMock("cust_q", "ccof_q", "sqpay_q")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	price := lookupPlan("go").Price

	for _, quoted := range []int64{price - 1, price + 1, 1} {
		_, err := Subscribe(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: "go", Subject: "quote-bound", ExpectedCents: quoted})
		var r *Refusal
		if !IsSaleRefused(err) || !errors.As(err, &r) || r.Code != "price_changed" {
			t.Fatalf("quoted %d against a price of %d answered %v; want a price_changed refusal", quoted, price, err)
		}
	}
	if m.chargeCalls != 0 || m.createCustomerCalls != 0 || m.addCardNonce != "" {
		t.Fatalf("the card was touched by a refused sale: charges=%d customers=%d nonce=%q", m.chargeCalls, m.createCustomerCalls, m.addCardNonce)
	}
	if n := len(subsOf(t, db, "quote-bound")); n != 0 {
		t.Fatalf("a refused sale opened %d subscription(s)", n)
	}

	// The price that was quoted sells, and a sale that states no quote still does.
	if _, err := Subscribe(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: "go", Subject: "quote-bound", ExpectedCents: price}); err != nil {
		t.Fatalf("the quoted price was refused: %v", err)
	}
	if m.chargeCalls != 1 {
		t.Fatalf("charges = %d; want one", m.chargeCalls)
	}
	org2 := moneyOrg("quote-none")
	withFakeSquare(t, squareMock("cust_n", "ccof_n", "sqpay_n"))
	if _, err := Subscribe(ctx, org2, SubscribeIn{SourceID: "cnon:ok", PlanID: "go", Subject: "quote-none"}); err != nil {
		t.Fatalf("a sale with no quote was refused: %v", err)
	}
}

// grantRail is the Stripe double that also reads shared payment tokens.
type grantRail struct{ *chargingRail }

func (grantRail) Grant(context.Context, string) (*processor.Grant, error) {
	return &processor.Grant{Active: true, Currency: "usd", MaxAmount: 1_000_000, ExpiresAt: time.Now().Add(time.Hour).Unix(), RecurringInterval: "month"}, nil
}

// An agent's plan checkout is charged at the total it quoted. A catalog that has moved since
// the quote is refused before any money moves, with a code the agent can act on, and the
// checkout stays usable: updating it quotes the new price.
func TestCompleteCheckout_APlanIsChargedAtItsQuoteOrNotAtAll(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db, _, ch := stripeBuyer(t, ctx, "agent-quote", "US", 1)
	rail := grantRail{ch}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(rail)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
	price := lookupPlan("go").Price
	pay := PaymentIn{Type: "shared_payment_token", Token: "spt_quote"}

	moved := "agent-moved"
	v, f := CreateCheckout(ctx, org, moved, "", []ItemIn{{Kind: "plan", Plan: "go"}})
	if f != nil || v.TotalCents != price {
		t.Fatalf("checkout = %+v %v", v, f)
	}
	// The catalog moves after the quote: the quote on record is not the price now.
	if _, _, err := checkoutsession.Change(v.ID, func(s *checkoutsession.Session) bool { s.TotalCents = price - 100; return true }); err != nil {
		t.Fatal(err)
	}
	out := CompleteCheckout(ctx, org, moved, "", v.ID, "k-moved", pay)
	if out.Status != 409 || out.View.Error == nil || out.View.Error.Code != "price_changed" || out.View.Order != nil {
		t.Fatalf("a moved price answered %d %+v", out.Status, out.View)
	}
	if len(ch.reqs) != 0 || len(subsOf(t, db, moved)) != 0 {
		t.Fatalf("money moved on a refused completion: %d charges, %d subscriptions", len(ch.reqs), len(subsOf(t, db, moved)))
	}
	// The checkout is usable: updating it quotes the price now, and that completes.
	if u, f := UpdateCheckout(ctx, org, moved, "", v.ID, []ItemIn{{Kind: "plan", Plan: "go"}}); f != nil || u.TotalCents != price {
		t.Fatalf("update = %+v %v", u, f)
	}
	if out = CompleteCheckout(ctx, org, moved, "", v.ID, "k-moved-2", pay); out.Status != 200 || out.View.Order == nil {
		t.Fatalf("the re-quoted checkout answered %d %+v", out.Status, out.View)
	}
	if len(ch.reqs) != 1 {
		t.Fatalf("charges = %d; want the one at the quote", len(ch.reqs))
	}
}
