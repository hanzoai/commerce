package checkout

import (
	"errors"

	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.com/api/checkout/balance"
	"crowdstart.com/api/checkout/null"
	"crowdstart.com/api/checkout/stripe"
	"crowdstart.com/models/cart"
	"crowdstart.com/models/multi"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/counter"
	"crowdstart.com/util/log"
)

func capture(c *gin.Context, org *organization.Organization, ord *order.Order) (*order.Order, error) {
	var err error
	var payments []*payment.Payment

	switch ord.Type {
	case "null":
		ord, payments, err = null.Capture(org, ord)
	case "balance":
		ord, payments, err = balance.Capture(org, ord)
	case "stripe":
		ord, payments, err = stripe.Capture(org, ord)
	case "paypal":
		payments = ord.Payments
	default:
		return nil, errors.New("Invalid order type")
	}

	if err != nil {
		return nil, err
	}

	ctx := ord.Context()

	updateOrder(ctx, ord, payments)

	if err := saveOrder(ctx, ord, payments); err != nil {
		return ord, err
	}

	saveRedemptions(ctx, ord)

	saveReferral(ctx, org, ord)

	updateCart(ctx, ord)

	updateStats(ctx, org, ord, payments)

	return ord, nil
}

func updateOrder(ctx appengine.Context, ord *order.Order, payments []*payment.Payment) {
	totalPaid := 0

	for _, pay := range payments {
		totalPaid += int(pay.Amount)
	}

	ord.Paid = currency.Cents(int(ord.Paid) + totalPaid)
	if ord.Paid == ord.Total {
		ord.PaymentStatus = payment.Paid
	}
}

func saveOrder(ctx appengine.Context, ord *order.Order, payments []*payment.Payment) error {
	vals := []interface{}{ord}

	for _, pay := range payments {
		vals = append(vals, pay)
	}

	return multi.Update(vals)
}

func saveRedemptions(ctx appengine.Context, ord *order.Order) {
	// Save coupon redemptions
	ord.GetCoupons()
	if len(ord.Coupons) > 0 {
		for _, coup := range ord.Coupons {
			if err := coup.SaveRedemption(); err != nil {
				log.Warn("Unable to save redemption: %v", err, ctx)
			}
		}
	}
}

func saveReferral(ctx appengine.Context, org *organization.Organization, ord *order.Order) {
	db := ord.Db

	// Referral
	if ord.ReferrerId != "" {
		ref := referrer.New(db)

		// if ReferrerId refers to non-existing token, then remove from order
		if err := ref.Get(ord.ReferrerId); err != nil {
			log.Warn("Order referenced non-existent referrer '%s'", ord.ReferrerId, ctx)
			ord.ReferrerId = ""
		} else {
			// Save referral
			rfl, err := ref.SaveReferral(ord.Id(), ord.UserId)
			if err != nil {
				log.Warn("Unable to save referral: %v", err, ctx)
			} else {
				// Update statistics
				if ref.AffiliateId != "" {
					if err := counter.IncrReferrerFees(ctx, org, ref.Id(), rfl); err != nil {
						log.Warn("Counter Error %s", err, ctx)
					}

					if err := counter.IncrAffiliateFees(ctx, org, ref.AffiliateId, rfl); err != nil {
						log.Warn("Counter Error %s", err, ctx)
					}
				}
			}
		}
	}
}

func updateCart(ctx appengine.Context, ord *order.Order) {
	// Update cart
	car := cart.New(ord.Db)

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
}

func updateStats(ctx appengine.Context, org *organization.Organization, ord *order.Order, payments []*payment.Payment) {
	log.Debug("Incrementing Counters? %v", ord.Test, ctx)
	if !ord.Test {
		log.Debug("Incrementing Counters", ctx)
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
}
