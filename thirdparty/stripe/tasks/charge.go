package tasks

import (
	"time"

	"github.com/stripe/stripe-go/charge"

	"appengine"
	"appengine/delay"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
)

// Update payment from charge
func UpdatePaymentFromCharge(pay *payment.Payment, ch *stripe.Charge) {
	pay.Status = payment.Unpaid

	// Update status
	if ch.Captured {
		pay.Status = payment.Paid
	}

	if ch.Status == "failed" {
		pay.Status = payment.Cancelled
	}

	if ch.Refunded {
		pay.Status = payment.Refunded
	}

	if ch.FraudDetails != nil {
		if ch.FraudDetails.UserReport == charge.ReportFraudulent ||
			ch.FraudDetails.StripeReport == charge.ReportFraudulent {
			pay.Status = payment.Fraudulent
		}
	}
}

// Synchronize payment using charge
var ChargeSync = delay.Func("stripe-charge-sync", func(ctx appengine.Context, ns string, token string, ch stripe.Charge, start time.Time) {
	ctx = getNamespace(ctx, ns)

	db := datastore.New(ctx)

	// Query by ancestor so we can use a transaction
	var payments []*payment.Payment
	keys, err := payment.Query(db).Filter("Account.ChargeId=", ch.ID).GetAll(&payments)
	if err != nil {
		log.Error("Failed to find payment for charge '%s': %v", ch.ID, err, ctx)
		return
	}

	// Find right payment and order
	var ord *order.Order
	var pay *payment.Payment

	for i, p := range payments {
		if p.Deleted || p.Test {
			continue
		}

		p.Mixin(db, p)

		// Try and get order
		ord = order.New(db)
		if err := ord.Get(p.OrderId); err != nil {
			continue
		}

		p.Parent = ord.Key()
		p.SetKey(keys[i])
		pay = p
		break
	}

	if pay == nil {
		log.Error("Unable to find payment for charge '%s': %v", ch.ID, err, ctx)
		return
	}

	// Update payment status
	UpdatePaymentFromCharge(pay, &ch)
	if err := pay.Put(); err != nil {
		log.Error("Unable to update payment %#v: %v", pay, err, ctx)
		return
	}

	// Update order with payment status
	ord.PaymentStatus = pay.Status
	if pay.Status == payment.Cancelled || pay.Status == payment.Refunded {
		ord.Status = order.Cancelled
	}

	if pay.Status == payment.Fraudulent {
		ord.Status = order.Locked
	}

	if err := ord.Put(); err != nil {
		log.Error("Unable to update payment %#v: %v", pay, err, ctx)
		return
	}
})
