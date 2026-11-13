package billing

import (
	"context"
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
)

// normalizeStripeWebhookEvent puts a verified Stripe delivery into the shape the
// settlement and clawback code reads (the same shape PayPal's and Square's are
// put in), so one set of effects serves every processor:
//
//   - payment.completed: a PaymentIntent that SUCCEEDED, keyed on its CHARGE id —
//     the reference every in-session charge stamps (stripe.Provider.Charge).
//   - refund.created / refund.updated: a Stripe refund, on the charge it returns.
//     It acts once it has SUCCEEDED.
//   - dispute.created / dispute.updated: a Stripe dispute, on the charge it
//     contests, in the vocabulary models/dispute orders. The event's time is the
//     dispute's version, so an older report never undoes a newer one.
//
// Any other event is dropped: its type is cleared, so it is recorded and
// acknowledged and moves no money. Stripe's subscription and invoice events are
// among them — commerce keeps the subscriptions; Stripe only moves the money.
//
// StripeWebhookEvents is the event list a Stripe webhook endpoint for commerce
// must subscribe to — the one list, for production and for every test-mode
// network, which universe's endpoint registration and the tests both read.
// charge.refunded is read too: it names the charge, and the refunds on it are
// read from Stripe (stripeRefunds), so a refund is taken back whether the endpoint
// delivers refund.created, refund.updated or only charge.refunded.
var StripeWebhookEvents = []string{
	"payment_intent.succeeded",
	"charge.refunded",
	"refund.created",
	"refund.updated",
	"charge.dispute.created",
	"charge.dispute.updated",
	"charge.dispute.closed",
}

func normalizeStripeWebhookEvent(evt *processor.WebhookEvent) {
	obj := evt.Data
	if obj == nil {
		evt.Type = ""
		return
	}
	live, _ := obj["stripe_livemode"].(bool)
	out := map[string]interface{}{"stripe_test": !live}
	copyKeys := func(keys ...string) {
		for _, k := range keys {
			if v, ok := obj[k]; ok {
				out[k] = v
			}
		}
	}

	switch evt.Type {
	case "payment.completed":
		charge := stringField(obj, "latest_charge")
		if stringField(obj, "status") != "succeeded" || charge == "" {
			evt.Type = ""
			break
		}
		out["id"], out["status"] = charge, "COMPLETED"
		out["payment_intent"] = stringField(obj, "id")
		copyKeys("amount", "currency")
	case "refund.succeeded": // charge.refunded
		out["charge"] = stringField(obj, "id")
		copyKeys("currency")
	case "refund.created", "refund.updated":
		out["id"], out["payment_id"] = stringField(obj, "id"), stringField(obj, "charge")
		out["status"] = "PENDING"
		if stringField(obj, "status") == "succeeded" {
			out["status"] = "COMPLETED"
		}
		if md, _ := obj["metadata"].(map[string]interface{}); md != nil && stringField(md, excessRefund) != "" {
			out[excessRefund] = true
		}
		copyKeys("amount", "currency")
	case "dispute.created", "dispute.updated":
		out["id"], out["payment_id"] = stringField(obj, "id"), stringField(obj, "charge")
		out["state"] = stripeDisputeState(stringField(obj, "status"))
		out["version"] = evt.Timestamp
		copyKeys("amount", "currency")
	default:
		evt.Type = ""
	}
	evt.Data = out
}

// stripeDisputeState is a Stripe dispute status in models/dispute's vocabulary.
// A dispute settled by refunding the charge withholds nothing here: the refund's
// own event takes the money back.
func stripeDisputeState(status string) string {
	switch strings.ToLower(status) {
	case "warning_needs_response":
		return "INQUIRY_EVIDENCE_REQUIRED"
	case "warning_under_review":
		return "INQUIRY_PROCESSING"
	case "warning_closed", "charge_refunded":
		return "INQUIRY_CLOSED"
	case "needs_response":
		return "EVIDENCE_REQUIRED"
	case "under_review":
		return "PROCESSING"
	case "won":
		return "WON"
	case "lost":
		return "LOST"
	}
	return ""
}

// stripeRefunds are the succeeded refunds on a charge, each as the refund event
// the clawback reads, so charge.refunded takes back exactly what refund.created
// would have: once per refund id.
func stripeRefunds(ctx context.Context, org *organization.Organization, chargeID string, test bool) ([]*processor.WebhookEvent, error) {
	sp, err := payment.ProcessorsForOrg(org).Existing(ctx, processor.Stripe)
	if err != nil {
		return nil, err
	}
	lr, ok := sp.(interface {
		ChargeRefunds(context.Context, string) ([]map[string]interface{}, error)
	})
	if !ok {
		return nil, fmt.Errorf("the Stripe processor cannot list refunds")
	}
	refunds, err := lr.ChargeRefunds(ctx, chargeID)
	if err != nil {
		return nil, err
	}
	out := make([]*processor.WebhookEvent, 0, len(refunds))
	for _, r := range refunds {
		if stringField(r, "status") != "succeeded" {
			continue
		}
		if md, _ := r["metadata"].(map[string]interface{}); md != nil && stringField(md, excessRefund) != "" {
			continue // an excess commerce returned itself, not a refund of the payment
		}
		out = append(out, &processor.WebhookEvent{
			Type: "refund.updated", Processor: processor.Stripe,
			Data: map[string]interface{}{
				"id": stringField(r, "id"), "payment_id": chargeID, "status": "COMPLETED",
				"amount": r["amount"], "currency": r["currency"], "stripe_test": test,
			},
		})
	}
	return out, nil
}
