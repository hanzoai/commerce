// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"strings"
	"time"

	"github.com/hanzoai/commerce/payment/processor"
)

// normalizeStripeWebhookEvent puts a verified Stripe delivery in the one shape
// the receiver's effects read (the shape Square delivers and PayPal is
// normalized to): a refund under data.refund, a dispute under data.dispute,
// each naming the payment by the id the charge recorded — the Charge (ch_…),
// which is what a Stripe charge answers as its ProcessorRef and what the
// payment index (models/paymentorg) is keyed on.
//
// The events Stripe must be sent (the endpoint's subscription in the Stripe
// dashboard): refund.created, refund.updated, charge.refund.updated,
// charge.dispute.created, charge.dispute.updated, charge.dispute.closed,
// charge.dispute.funds_withdrawn, charge.dispute.funds_reinstated.
//
// Every other event is acknowledged and moves nothing: it is renamed under
// "stripe." so no settlement, subscription or clawback effect reads it. In
// particular payment_intent.succeeded never credits anything — every Stripe
// charge this service takes is confirmed in the request that takes it, and
// credited there, keyed on the charge; a second, webhook-driven credit keyed on
// the PaymentIntent would be the same money twice.
func normalizeStripeWebhookEvent(evt *processor.WebhookEvent) {
	if evt == nil {
		return
	}
	if evt.Data == nil {
		evt.Data = map[string]interface{}{}
	}
	raw := evt.Type
	evt.Data["stripe_event_type"] = raw
	obj := evt.Data
	created, _ := numberField(obj, "stripe_created")
	money := map[string]interface{}{"currency": strings.ToUpper(stringField(obj, "currency"))}
	if amt, ok := numberField(obj, "amount"); ok {
		money["amount"] = amt
	}

	switch raw {
	case "refund.created", "refund.updated", "charge.refund.updated":
		status := strings.ToUpper(stringField(obj, "status"))
		if status == "SUCCEEDED" {
			status = "COMPLETED"
		}
		evt.Type = "refund.updated"
		evt.Data["refund"] = map[string]interface{}{
			"id":           stringField(obj, "id"),
			"payment_id":   stringField(obj, "charge"),
			"status":       status,
			"amount_money": money,
		}
	case "charge.dispute.created", "charge.dispute.updated", "charge.dispute.closed",
		"charge.dispute.funds_withdrawn", "charge.dispute.funds_reinstated":
		d := map[string]interface{}{
			"id":           stringField(obj, "id"),
			"payment_id":   stringField(obj, "charge"),
			"state":        stripeDisputeState(stringField(obj, "status")),
			"amount_money": money,
		}
		if created > 0 {
			// Stripe disputes carry no version; the event's own time orders them.
			d["version"] = created
			d["updated_at"] = time.Unix(created, 0).UTC().Format(time.RFC3339)
		}
		evt.Type = "dispute.state.updated"
		evt.Data["dispute"] = d
	default:
		evt.Type = "stripe." + raw
	}
}

// stripeDisputeState names a Stripe dispute status in the processor-neutral
// states models/dispute reads: an inquiry ("warning_…") withholds nothing, a
// chargeback withholds the funds until it is decided.
func stripeDisputeState(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "warning_needs_response":
		return "INQUIRY_EVIDENCE_REQUIRED"
	case "warning_under_review":
		return "INQUIRY_PROCESSING"
	case "warning_closed":
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
	return strings.ToUpper(status)
}
