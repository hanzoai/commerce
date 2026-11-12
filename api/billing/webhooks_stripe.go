package billing

import (
	"strings"

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
	case "refund.created", "refund.updated":
		out["id"], out["payment_id"] = stringField(obj, "id"), stringField(obj, "charge")
		out["status"] = "PENDING"
		if stringField(obj, "status") == "succeeded" {
			out["status"] = "COMPLETED"
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
