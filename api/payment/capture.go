package payment

import (
	"github.com/gin-gonic/gin"

	aeds "appengine/datastore"

	"crowdstart.com/api/payment/balance"
	"crowdstart.com/api/payment/stripe"
	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/thirdparty/redis"
	"crowdstart.com/util/log"
)

func capture(c *gin.Context, org *organization.Organization, ord *order.Order) (*order.Order, error) {
	var err error
	var payments []*payment.Payment
	var keys []*aeds.Key

	// We could actually capture different types of things here...
	switch ord.Type {
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
	ord.MustPut()

	db := datastore.New(ord.Db.Context)
	if _, err = db.PutMulti(keys, payments); err != nil {
		return nil, err
	}

	ctx := db.Context

	t := ord.CreatedAt
	if err := redis.IncrTotalOrders(ctx, org, t); err != nil {
		log.Warn("Redis Error %s", err, ctx)
	}
	if err := redis.IncrTotalSales(ctx, org, payments, t); err != nil {
		log.Warn("Redis Error %s", err, ctx)
	}
	if err := redis.IncrTotalProductOrders(ctx, org, ord, t); err != nil {
		log.Warn("Redis Error %s", err, ctx)
	}

	if ord.StoreId != "" {
		if err := redis.IncrStoreOrders(ctx, org, ord.StoreId, t); err != nil {
			log.Warn("Redis Error %s", err, ctx)
		}
		if err := redis.IncrStoreSales(ctx, org, ord.StoreId, payments, t); err != nil {
			log.Warn("Redis Error %s", err, ctx)
		}
		if err := redis.IncrStoreProductOrders(ctx, org, ord.StoreId, ord, t); err != nil {
			log.Warn("Redis Error %s", err, ctx)
		}
	}

	if ord.StoreId != "" {
	}

	// Need to figure out a way to count coupon uses
	return ord, nil
}
