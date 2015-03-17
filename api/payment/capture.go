package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/thirdparty/stripe2"
)

func capture(c *gin.Context, org *organization.Organization, ord *order.Order) (*order.Order, error) {
	// Get namespaced context off order
	ctx := ord.Db.Context

	// Get client we can use for API calls
	client := stripe.New(ctx, org.Stripe.AccessToken)

	// Capture any uncaptured payments
	for _, payment := range ord.Payments {
		if !payment.Captured {
			client.Capture(payment.ChargeId)
		}
	}

	return ord, nil
}
