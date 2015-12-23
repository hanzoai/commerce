package stripe

import (
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
)

// Refunds the entire order
func Refund(org *organization.Organization, ord *order.Order, refundAmount uint64) error {
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
		_, err := client.RefundPayment(p, currency.Cents(refundAmount))
		if err != nil {
			return err
		}
	}

	return nil
}
