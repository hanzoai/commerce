package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/api/payment/stripe"
	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/referralinstance"
	"crowdstart.com/models/types/currency"
)

func capture(c *gin.Context, org *organization.Organization, ord *order.Order) (*order.Order, error) {
	// We could actually capture different types of things here...
	ord, keys, payments, err := stripe.Capture(org, ord)
	if err != nil {
		return nil, err
	}

	// Referral
	if ord.ReferralInstanceId != "" {
		db := datastore.New(c)
		ri := referralinstance.New(db)

		// if ReferralInstanceId refers to non-existing token, then remove from order
		if err = ri.GetById(ord.ReferralInstanceId); err != nil {
			ord.ReferralInstanceId = ""
		} else {
			ri.ReferredOrderIds = append(ri.ReferredOrderIds, ord.Id())
			if err = ri.Put(); err != nil {
				return nil, err
			}

			if _, err = ri.ApplyBonus(); err != nil {
				return nil, err
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
