package checkout

import (
	"github.com/gin-gonic/gin"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/api/checkout/balance"
	"hanzo.io/api/checkout/stripe"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/referrer"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/log"
)

func capture(c *gin.Context, org *organization.Organization, ord *order.Order) (*order.Order, error) {
	var err error
	var payments []*payment.Payment
	var keys []*aeds.Key

	// We could actually capture different types of things here...
	switch ord.Type {
	case "paypal":
	case "balance":
		ord, keys, payments, err = balance.Capture(org, ord)
		if err != nil {
			return nil, err
		}
	default:
		ord, keys, payments, err = stripe.Capture(org, ord)
		if err != nil {
			return nil, err
		}
	}

	return CompleteCapture(c, org, ord, keys, payments)
}

func CompleteCapture(c *gin.Context, org *organization.Organization, ord *order.Order, keys []*aeds.Key, payments []*payment.Payment) (*order.Order, error) {
	var err error

	db := ord.Db

	log.Debug("Completing Capture for\nOrder %v\nPayments %v", ord, payments, c)

	// Referral
	if ord.ReferrerId != "" {
		ref := referrer.New(ord.Db)

		// if ReferrerId refers to non-existing token, then remove from order
		if err = ref.GetById(ord.ReferrerId); err != nil {
			ord.ReferrerId = ""
		} else {
			// Try to save referral, save updated referrer
			if _, err := ref.SaveReferral(ord); err != nil {
				log.Warn("Unable to save referral: %v", err, c)
			}
		}
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
	vals := make([]interface{}, len(payments))
	for i := range payments {
		vals[i] = payments[i]
	}

	akey, _ := ord.Key().(*aeds.Key)
	keys = append(keys, akey)
	vals = append(vals, ord)

	if _, err = db.PutMulti(keys, vals); err != nil {
		return nil, err
	}

	// Need to figure out a way to count coupon uses
	return ord, nil
}
