package tasks

import (
	"context"
	"time"

	"github.com/stripe/stripe-go/v84"

	"github.com/hanzoai/commerce/delay"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/types/currency"
	hanzo_stripe "github.com/hanzoai/commerce/thirdparty/stripe"
)

// Update payment from charge
func UpdatePaymentFromCharge(pay *payment.Payment, ch *hanzo_stripe.Charge) {
	pay.Amount = currency.Cents(ch.Amount)
	pay.AmountRefunded = currency.Cents(ch.AmountRefunded)

	pay.Status = payment.Unpaid

	// Update status
	if ch.Captured {
		pay.Status = payment.Paid
	}

	if ch.Status == "failed" {
		pay.Status = payment.Failed
	}

	if ch.Refunded {
		pay.Status = payment.Refunded
	}

	if ch.FraudDetails != nil {
		if ch.FraudDetails.UserReport == stripe.ChargeFraudUserReportSafe || ch.FraudDetails.StripeReport == stripe.ChargeFraudStripeReportFraudulent {
			pay.Status = payment.Fraudulent
		}
	}
}

// Synchronize payment using charge
var ChargeSync = delay.Func("stripe-charge-sync", func(ctx context.Context, ns string, token string, ch hanzo_stripe.Charge, start time.Time) {
	log.Debug("Charge %s", ch, ctx)

	ctx = getNamespacedContext(ctx, ns)

	// Get payment using charge
	pay, ok, err := getPaymentFromCharge(ctx, &ch)
	if err != nil {
		log.Error("Failed to query for payment associated with charge '%s', namespace: '%s': %v\n%#v", ch.ID, err, ch, ctx)
		return
	}

	log.Debug("Payment Id: %v from ChargeId: %v", pay.Id(), ch.ID, ctx)

	if !ok {
		log.Warn("No payment associated with charge '%s'", ch.ID, ctx)
		return
	}

	if start.Before(pay.UpdatedAt) {
		log.Warn("Payment '%s' previously updated, bailing out", pay.Id(), ctx)
		return
	}

	fees, err := pay.GetFees()
	if err != nil {
		log.Error("Failed to query for fees associated with charge '%s': %v", ch.ID, err, ctx)
		return
	}

	// Update payment using charge
	err = pay.RunInTransaction(func() error {
		log.Debug("Payment before: %+v", pay, ctx)
		UpdatePaymentFromCharge(pay, &ch)
		log.Debug("Payment after: %+v", pay, ctx)
		log.Debug("Fees before: %+v", fees, ctx)
		UpdateFeesFromPayment(fees, pay)
		log.Debug("Fees after: %+v", fees, ctx)

		return pay.Put()
	}, nil)

	if err != nil {
		log.Error("Failed to update payment '%s' from charge %v: ", pay.Id(), ch, err, ctx)
		return
	}

	// Update charge if necessary
	updateChargeFromPayment(ctx, token, pay, &ch)

	// Update order
	if pay.OrderId == "" {
		log.Warn("No order associated with payment: %+v", pay, ctx)
		return
	}

	updateOrder.Call(ctx, ns, pay.OrderId, pay.AmountRefunded, start)
})
