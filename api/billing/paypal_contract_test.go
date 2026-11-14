package billing

import (
	"github.com/hanzoai/commerce/payment/processor"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/providers/paypal/paypaltest"
	types "github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// These run the real PayPal provider, through the real per-org registry
// (payment.ProcessorsForOrg reading the deployment's sandbox credentials), over
// HTTP to PayPal's contract (paypaltest): the spec's limits, PayPal's recorded
// refusals, PayPal's published success bodies.

func paypalContractOrg(t *testing.T) (*paypaltest.Server, *organization.Organization, *ae.Context) {
	t.Helper()
	t.Cleanup(processor.EnvSeed{"PAYPAL_SANDBOX_CLIENT_ID": "contract-client", "PAYPAL_SANDBOX_CLIENT_SECRET": "contract-secret"}.Apply())
	srv := paypaltest.New(t, "contract-client", "contract-secret")
	srv.Route(t)
	injectLedger(t, newFakeLedger())

	ctx := ae.NewContext()
	t.Cleanup(ctx.Close)
	org := organization.New(datastore.New(ctx))
	org.Name = "hanzo"
	org.MustCreate()

	pl := plan.New(plan.AuthorityDB(ctx))
	pl.Slug = "pro-3x"
	pl.Name = "Pro"
	pl.Price = currency.Cents(1500)
	pl.Currency = currency.USD
	pl.Interval = types.Monthly
	pl.IntervalCount = 1
	pl.Category = "personal"
	pl.Status = plan.StatusActive
	pl.Managed = true
	pl.MustCreate()
	return srv, org, &ctx
}

// THE hanzo.ai/pay PAYPAL FAILURE. A pay-first buyer is a guest wallet
// ("hanzo/guest-" + 12 hex) and the checkout sends no idempotency key, so the
// order's PayPal-Request-Id is composed from the plan's facts — 129 characters
// for this plan. PayPal refuses one longer than its limit with 422
// INVALID_STRING_LENGTH and opens no order, which commerce answered as 502
// "failed to create PayPal order": the Smart Button's createOrder failed every
// time. Fitted at PayPal's boundary, the same order opens.
func TestPayPalContract_GuestPlanOrderOpens(t *testing.T) {
	srv, org, ctx := paypalContractOrg(t)

	out, f := CreatePayPalOrder(*ctx, org, PayPalOrderIn{
		PlanID:   "pro-3x",
		Interval: types.Monthly,
		Subject:  "hanzo/guest-0123456789ab",
	})
	if f != nil {
		t.Fatalf("guest plan PayPal order: %d %s (%v)", f.Status, f.Message, f.Err)
	}
	if out.Intent != "AUTHORIZE" || out.AmountCents != 1500 || out.Currency != "usd" {
		t.Fatalf("order = %+v; want AUTHORIZE 1500 usd", out)
	}
	o := srv.Order(out.OrderID)
	if o == nil || o.Status != "CREATED" || o.Value != "15.00" || o.Currency != "USD" || o.CustomID != "hanzo/guest-0123456789ab" {
		t.Fatalf("PayPal holds %+v", o)
	}
	for _, id := range srv.RequestIDs {
		if len(id) == 0 || len(id) > srv.Limit("PayPal-Request-Id") {
			t.Fatalf("PayPal-Request-Id %q is %d characters; PayPal takes 1..%d", id, len(id), srv.Limit("PayPal-Request-Id"))
		}
	}

	// The same order asked for again in the same window is the same order: the
	// fitted key is deterministic, so PayPal replays rather than opening a second.
	again, f := CreatePayPalOrder(*ctx, org, PayPalOrderIn{PlanID: "pro-3x", Interval: types.Monthly, Subject: "hanzo/guest-0123456789ab"})
	if f != nil || again.OrderID != out.OrderID {
		t.Fatalf("retry opened %+v (%v); want the replay of %s", again, f, out.OrderID)
	}
}

// The plan order through to the money: the buyer approves on PayPal, the sale
// authorizes the approved order and captures the authorization, and the
// subscription opens on what PayPal captured.
func TestPayPalContract_PlanOrderAuthorizeCapture(t *testing.T) {
	srv, org, ctx := paypalContractOrg(t)

	order, f := CreatePayPalOrder(*ctx, org, PayPalOrderIn{PlanID: "pro-3x", Interval: types.Monthly, Subject: "hanzo/guest-0123456789ab"})
	if f != nil {
		t.Fatalf("CreatePayPalOrder: %d %s", f.Status, f.Message)
	}
	srv.Approve(order.OrderID)

	sale, err := Subscribe(*ctx, org, SubscribeIn{
		SourceID:       order.OrderID,
		PlanID:         "pro-3x",
		Interval:       types.Monthly,
		Subject:        "hanzo/guest-0123456789ab",
		Provider:       "paypal",
		Wallet:         true,
		IdempotencyKey: "guest-sale-" + order.OrderID,
	})
	if err != nil {
		t.Fatalf("Subscribe with the approved PayPal order: %v", err)
	}
	if sale.SubscriptionID == "" || sale.AmountCents != 1500 {
		t.Fatalf("sale = %+v; want a subscription paid 1500", sale)
	}
	if o := srv.Order(order.OrderID); o.Status != "COMPLETED" {
		t.Fatalf("PayPal order is %s; want COMPLETED", o.Status)
	}
}

// A top-up order through to the money: CAPTURE order, the buyer approves, the
// capture credits the balance. An order the buyer never approved is refused by
// PayPal and credits nothing.
func TestPayPalContract_TopupCreateApproveCapture(t *testing.T) {
	srv, org, ctx := paypalContractOrg(t)

	order, f := CreatePayPalOrder(*ctx, org, PayPalOrderIn{AmountCents: 2500, Currency: "usd", Subject: "hanzo/alice"})
	if f != nil {
		t.Fatalf("CreatePayPalOrder top-up: %d %s", f.Status, f.Message)
	}
	if order.Intent != "CAPTURE" {
		t.Fatalf("top-up order intent %s; want CAPTURE", order.Intent)
	}

	// Not approved yet: PayPal says ORDER_NOT_APPROVED and nothing is credited.
	if _, pf := TakePayment(*ctx, org, TakePaymentIn{
		SourceID: order.OrderID, AmountCents: 2500, Currency: "usd", Subject: "hanzo/alice",
		IdempotencyKey: "topup-unapproved", Provider: "paypal",
	}); pf == nil || pf.Status != 402 {
		t.Fatalf("capture of an unapproved order = %+v; want 402", pf)
	}

	srv.Approve(order.OrderID)
	paid, pf := TakePayment(*ctx, org, TakePaymentIn{
		SourceID: order.OrderID, AmountCents: 2500, Currency: "usd", Subject: "hanzo/alice",
		IdempotencyKey: "topup-approved", Provider: "paypal",
	})
	if pf != nil {
		t.Fatalf("capture of the approved order: %d %s", pf.Status, pf.Message)
	}
	if paid.ProcessorRef != order.OrderID+"-capture" {
		t.Fatalf("processor ref %q; want PayPal's capture id", paid.ProcessorRef)
	}
	if o := srv.Order(order.OrderID); o.Status != "COMPLETED" {
		t.Fatalf("PayPal order is %s; want COMPLETED", o.Status)
	}
}
