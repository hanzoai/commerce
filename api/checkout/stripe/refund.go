package stripe

import (
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
)

// Refunds the entire order
func Refund(org *organization.Organization, ord *order.Order) error {
	// Get namespaced context off order
	db := ord.Db
	ctx := db.Context

	// Get client we can use for API calls
	client := stripe.New(ctx, org.StripeToken())

	payments := make([]*payment.Payment, 0)
	_, err := payment.Query(db).Ancestor(ord.Key()).GetAll(&payments)
	if err != nil {
		return err
	}

	log.Debug("payments %v", payments)
	// Capture any uncaptured payments
	for _, p := range payments {
		_, err := client.RefundEntirePayment(p)
		if err != nil {
			return err
		}
	}

	return nil
}
