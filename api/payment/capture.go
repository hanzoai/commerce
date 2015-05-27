package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/api/payment/stripe"
	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/log"
)

func capture(c *gin.Context, org *organization.Organization, ord *order.Order) (*order.Order, error) {
	// We could actually capture different types of things here...
	ord, keys, payments, err := stripe.Capture(org, ord)
	if err != nil {
		return nil, err
	}

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
	ord.Put()

	db := datastore.New(ord.Db.Context)
	if _, err = db.PutMulti(keys, payments); err != nil {
		return nil, err
	}

	return ord, nil
}
