package billing

import (
	"testing"

	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A completion that carries the Quote its caller verified is taken only if the checkout is
// still that line at that total. A buyer who changes the checkout between the caller's check
// and the completion, as UpdateCheckout lets them, is refused before any money moves, and the
// checkout stays usable.
func TestCompleteCheckout_AQuotedLineIsChargedAsQuotedOrNotAtAll(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, _, _, ch := stripeBuyer(t, ctx, "agent-changed", "US", 1)
	rail := grantRail{ch}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(rail)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
	min, _ := topupBounds()

	v, f := CreateCheckout(ctx, org, "agent-changed", "", []ItemIn{{Kind: "credit", UsdCents: min}})
	if f != nil {
		t.Fatal(f)
	}
	quote := Quote{Kind: "credit", UsdCents: min, TotalCents: min}
	// The buyer swaps the line for a larger one after the caller verified it.
	if _, f := UpdateCheckout(ctx, org, "agent-changed", "", v.ID, []ItemIn{{Kind: "credit", UsdCents: 2 * min}}); f != nil {
		t.Fatal(f)
	}
	pay := PaymentIn{Type: "shared_payment_token", Token: "spt_changed", Quote: quote}
	out := CompleteCheckout(ctx, org, "agent-changed", "", v.ID, "k-1", pay)
	if out.Status != 409 || out.View.Error == nil || out.View.Error.Code != "checkout_changed" || out.View.Order != nil {
		t.Fatalf("a changed checkout answered %d %+v", out.Status, out.View)
	}
	if len(ch.reqs) != 0 {
		t.Fatalf("money moved on a refused completion: %d charges", len(ch.reqs))
	}
	// A quote of another total, of another kind, and of a plan are each not this checkout.
	for _, q := range []Quote{
		{Kind: "credit", UsdCents: 2 * min, TotalCents: min},
		{Kind: "plan", Plan: "go", TotalCents: 2 * min},
		{Kind: "credit", UsdCents: min, TotalCents: 2 * min},
	} {
		pay.Quote = q
		if out = CompleteCheckout(ctx, org, "agent-changed", "", v.ID, "k-2", pay); out.View.Error == nil || out.View.Error.Code != "checkout_changed" {
			t.Fatalf("quote %+v answered %d %+v", q, out.Status, out.View)
		}
	}
	if s, _, _ := checkoutsession.Get(v.ID); s.Status != checkoutsession.Failed {
		t.Fatalf("the refused checkout is %s; it stays usable as failed", s.Status)
	}
	// The line as it now is, quoted, is charged; and a completion with no quote still is.
	pay.Quote = Quote{Kind: "credit", UsdCents: 2 * min, TotalCents: 2 * min}
	if out = CompleteCheckout(ctx, org, "agent-changed", "", v.ID, "k-3", pay); out.Status != 200 || out.View.Order == nil {
		t.Fatalf("the checkout as quoted answered %d %+v", out.Status, out.View)
	}
	if len(ch.reqs) != 1 || int64(ch.reqs[0].Amount) != 2*min {
		t.Fatalf("charges = %+v; want the one at the quote", ch.reqs)
	}
	w, f := CreateCheckout(ctx, org, "agent-changed", "", []ItemIn{{Kind: "credit", UsdCents: min}})
	if f != nil {
		t.Fatal(f)
	}
	if out = CompleteCheckout(ctx, org, "agent-changed", "", w.ID, "k-4", PaymentIn{Type: "shared_payment_token", Token: "spt_changed"}); out.Status != 200 {
		t.Fatalf("a completion with no quote answered %d %+v", out.Status, out.View)
	}
}
