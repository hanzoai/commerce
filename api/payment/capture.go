package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/api/payment/stripe"
	"crowdstart.io/datastore"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
)

func capture(c *gin.Context, org *organization.Organization, ord *order.Order) (*order.Order, error) {
	// We could actually capture different types of things here...
	ord, keys, payments, err := stripe.Capture(org, ord)
	if err != nil {
		return nil, err
	}

	// Save order and payments
	ord.Put()

	db := datastore.New(org.Namespace(c))

	for i, payment := range payments {
		db.PutKind(payment.Kind(), keys[i], payment)
	}

	return ord, nil
}
