package checkout

import (
	"github.com/gin-gonic/gin"

	aeds "appengine/datastore"

	"crowdstart.com/api/checkout/balance"
	"crowdstart.com/api/checkout/stripe"
	"crowdstart.com/models/cart"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/thirdparty/mailchimp"
	"crowdstart.com/util/counter"
	"crowdstart.com/util/log"
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
		ref := referrer.New(db)

		// if ReferrerId refers to non-existing token, then remove from order
		if err = ref.Get(ord.ReferrerId); err != nil {
			ord.ReferrerId = ""
		} else {
			// Try to save referral, save updated referrer
			if _, err := ref.SaveReferral(ord.Id(), ord.UserId); err != nil {
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

	ctx := db.Context

	// Save coupon redemptions
	ord.GetCoupons()
	if len(ord.Coupons) > 0 {
		for _, coup := range ord.Coupons {
			if err := coup.SaveRedemption(); err != nil {
				log.Warn("Unable to save redemption: %v", err, ctx)
			}
		}
	}

	// Update cart
	car := cart.New(db)

	if ord.CartId != "" {
		if err := car.GetById(ord.CartId); err != nil {
			log.Warn("Unable to find cart: %v", err, ctx)
		} else {
			car.Status = cart.Ordered
			if err := car.Update(); err != nil {
				log.Warn("Unable to save cart: %v", err, ctx)
			}
		}
	}

	// Mailchimp shenanigans
	if org.Mailchimp.APIKey != "" {
		// Create new mailchimp client (used everywhere else)
		client := mailchimp.New(ctx, org.Mailchimp.APIKey)

		client.DeleteCart(org.DefaultStore, car)
		client.CreateOrder(org.DefaultStore, ord)

		// Just get buyer off first payment
		if err := client.SubscribeCustomer(org.Mailchimp.ListId, payments[0].Buyer); err != nil {
			log.Warn("Failed to subscribe '%s' to Mailchimp list '%s': %v", payments[0].Buyer.Email, org.Mailchimp.ListId, err)
		}
	}

	log.Debug("Incrementing Counters? %v", ord.Test, c)
	if !ord.Test {
		log.Debug("Incrementing Counters", c)
		t := ord.CreatedAt
		if err := counter.IncrTotalOrders(ctx, org, t); err != nil {
			log.Warn("Counter Error %s", err, ctx)
		}
		if err := counter.IncrTotalSales(ctx, org, payments, t); err != nil {
			log.Warn("Counter Error %s", err, ctx)
		}
		if err := counter.IncrTotalProductOrders(ctx, org, ord, t); err != nil {
			log.Warn("Counter Error %s", err, ctx)
		}

		if ord.StoreId != "" {
			if err := counter.IncrStoreOrders(ctx, org, ord.StoreId, t); err != nil {
				log.Warn("Counter Error %s", err, ctx)
			}
			if err := counter.IncrStoreSales(ctx, org, ord.StoreId, payments, t); err != nil {
				log.Warn("Counter Error %s", err, ctx)
			}
			if err := counter.IncrStoreProductOrders(ctx, org, ord.StoreId, ord, t); err != nil {
				log.Warn("Counter Error %s", err, ctx)
			}
		}
	}

	// Need to figure out a way to count coupon uses
	return ord, nil
}
