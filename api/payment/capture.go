package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/thirdparty/stripe2"

	. "crowdstart.io/models2"
)

func capture(c *gin.Context, org *organization.Organization, ord *order.Order) (*order.Order, error) {
	// Get namespaced context off order
	ctx := ord.Db.Context

	// Get client we can use for API calls
	client := stripe.New(ctx, org.Stripe.AccessToken)

	// Capture any uncaptured payments
	for i, payment := range ord.Payments {
		if !payment.Captured {
			ch, err := client.Capture(payment.ChargeId)

			// Charge failed for some reason, bail
			if err != nil {
				return nil, err
			}
			if !ch.Captured {
				return nil, FailedToCaptureCharge
			}

			// Update payment
			payment.Captured = true
			payment.Status = PaymentPaid
			payment.Amount = Cents(ch.Amount)
			payment.AmountRefunded = Cents(ch.AmountRefunded)
			ord.Payments[i] = payment
		}
	}

	// Save order
	ord.Put()

	return ord, nil
}
