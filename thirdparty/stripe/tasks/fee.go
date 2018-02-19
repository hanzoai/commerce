package tasks

// Synchronize payment using charge
import (
	"time"

	"google.golang.org/appengine"

	"hanzo.io/thirdparty/stripe"
	"hanzo.io/util/delay"
	"hanzo.io/log"
)

var FeeSync = delay.Func("stripe-fee-sync", func(ctx context.Context, ns string, token string, ch stripe.Charge, start time.Time) {
	log.Debug("Fee Sync %s", ch, ctx)

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
		log.Debug("Payment after: %+v", pay, ctx)
		UpdateFeesFromPayment(fees, pay)

		return pay.Put()
	})

	if err != nil {
		log.Error("Failed to update fees '%s' from charge %v: ", pay.Id(), ch, err, ctx)
		return
	}
})
