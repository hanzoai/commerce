package tasks

import (
	"appengine"

	"crowdstart.com/datastore"
	"crowdstart.com/models/cart"
	"crowdstart.com/models/multi"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/mailchimp"
	"crowdstart.com/util/counter"
	"crowdstart.com/util/delay"
	"crowdstart.com/util/emails"
	"crowdstart.com/util/log"

	. "crowdstart.com/models"
)

var CaptureAsync = delay.Func("capture-async", func(ctx appengine.Context, orgId string, ordId string) {
	db := datastore.New(ctx)
	org := organization.New(db)
	ord := order.New(db)

	org.MustGetById(orgId)
	ord.MustGetById(ordId)

	payments := make([]*payment.Payment, 0)
	if _, err := payment.Query(db).Ancestor(ord.Key()).GetAll(payments); err != nil {
		log.Error("Unable to find payments associated with order '%s'", ord.Id())
	}

	sendOrderConfirmation(ctx, org, ord, payments[0].Buyer)
	saveRedemptions(ctx, ord)
	saveReferral(ctx, org, ord)
	updateCart(ctx, ord)
	updateStats(ctx, org, ord, payments)
	updateMailchimp(ctx, org, ord)
})

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

func sendOrderConfirmation(ctx appengine.Context, org *organization.Organization, ord *order.Order, buyer Buyer) {
	// Send Create user
	usr := new(user.User)
	usr.Email = buyer.Email
	usr.FirstName = buyer.FirstName
	usr.LastName = buyer.LastName
	emails.SendOrderConfirmationEmail(ctx, org, ord, usr)
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

	// Check for referrer
	if ord.ReferrerId == "" {
		return // No referrer
	}

	// Search for referrer
	ref := referrer.New(db)
	if err := ref.GetById(ord.ReferrerId); err != nil {
		log.Warn("Order referenced non-existent referrer '%s'", ord.ReferrerId, ctx)
		ord.ReferrerId = ""
		return
	}

	// Save referral
	rfl, err := ref.SaveReferral(ctx, org.Id(), referral.NewOrder, ord)
	if err != nil {
		log.Warn("Unable to save referral: %v", err, ctx)
		return
	}

	if err := counter.IncrReferrerFees(ctx, org, ref.Id(), rfl); err != nil {
		log.Warn("Counter Error %s", err, ctx)
	}

	// Update statistics
	if ref.AffiliateId != "" {
		if err := counter.IncrAffiliateFees(ctx, org, ref.AffiliateId, rfl); err != nil {
			log.Warn("Counter Error %s", err, ctx)
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

func updateMailchimp(ctx appengine.Context, org *organization.Organization, ord *order.Order) {
	// Save user as customer in Mailchimp if configured
	if org.Mailchimp.APIKey != "" {
		// Create new mailchimp client
		client := mailchimp.New(ctx, org.Mailchimp.APIKey)

		// Update cart
		car := cart.New(ord.Db)

		if ord.CartId != "" {
			if err := car.GetById(ord.CartId); err != nil {
				log.Warn("Unable to find cart: %v", err, ctx)
			} else {
				// Delete cart in mailchimp
				if err := client.DeleteCart(org.DefaultStore, car); err != nil {
					log.Warn("Failed to delete Mailchimp cart: %v", err, ctx)
				}
			}
		}

		// Create order in mailchimp
		if err := client.CreateOrder(org.DefaultStore, ord); err != nil {
			log.Warn("Failed to create Mailchimp order: %v", err, ctx)
		}
	}
}
