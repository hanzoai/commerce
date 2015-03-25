package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/payment"
	"crowdstart.io/thirdparty/stripe2"

	. "crowdstart.io/models2"
)

func capture(c *gin.Context, org *organization.Organization, ord *order.Order) (*order.Order, error) {
	// Get namespaced context off order
	db := ord.Db
	ctx := db.Context

	// Get client we can use for API calls
	client := stripe.New(ctx, org.Stripe.AccessToken)

	payments := make([]*payment.Payment, 0)
	payment.Query(db).Ancestor(ord.Key()).GetAll(payments)

	// Capture any uncaptured payments
	for _, p := range payments {
		if !p.Captured {
			ch, err := client.Capture(p.ChargeId)

			// Charge failed for some reason, bail
			if err != nil {
				return nil, err
			}
			if !ch.Captured {
				return nil, FailedToCaptureCharge
			}

			// Update payment
			p.Captured = true
			p.Status = payment.Paid
			p.Amount = Cents(ch.Amount)
			p.AmountRefunded = Cents(ch.AmountRefunded)
		}
	}

	// Save order
	ord.Put()

	return ord, nil
}
