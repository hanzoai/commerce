package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/api/payment/stripe"
	"crowdstart.io/datastore"
	"crowdstart.io/models/order"
	"crowdstart.io/models/organization"
	"crowdstart.io/models/payment"
	"crowdstart.io/models/types/currency"
)

func capture(c *gin.Context, org *organization.Organization, ord *order.Order) (*order.Order, error) {
	// We could actually capture different types of things here...
	ord, keys, payments, err := stripe.Capture(org, ord)
	if err != nil {
		return nil, err
	}

	// Update amount paid
	totalPaid := 0
	for _, pay := range payments {
		totalPaid += int(pay.Amount)
	}

	ord.Paid = currency.Cents(int(ord.Paid) + totalPaid)
	if ord.Paid == ord.Total {
		ord.PaymentStatus = payment.Paid
	}

	// Save order and payments
	ord.Put()

	db := datastore.New(ord.Db.Context)
	if _, err = db.PutMulti(keys, payments); err != nil {
		return nil, err
	}

	return ord, nil
}
