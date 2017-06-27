package tasks

import (
	"appengine"

	"hanzo.io/datastore"
	"hanzo.io/models/cart"
	"hanzo.io/models/multi"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/referral"
	"hanzo.io/models/referrer"
	"hanzo.io/models/store"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/mailchimp"
	"hanzo.io/util/counter"
	"hanzo.io/util/delay"
	"hanzo.io/util/emails"
	"hanzo.io/util/log"

	. "hanzo.io/models"
)

var CaptureAsync = delay.Func("capture-async", func(ctx appengine.Context, orgId string, ordId string) {
	db := datastore.New(ctx)
	org := organization.New(db)
	nsdb := datastore.New(org.Namespaced(ctx))
	ord := order.New(nsdb)
	usr := user.New(nsdb)

	org.MustGetById(orgId)
	ord.MustGetById(ordId)
	usr.MustGetById(ord.UserId)

	updateMailchimp(ctx, org, ord, usr)

	// payments := make([]*payment.Payment, 0)
	// if _, err := payment.Query(nsdb).Ancestor(ord.Key()).GetAll(payments); err != nil {
	// 	log.Error("Unable to find payments associated with order '%s'", ord.Id())
	// }

	// sendOrderConfirmation(ctx, org, ord, payments[0].Buyer)
	// saveRedemptions(ctx, ord)
	// saveReferral(ctx, org, ord)
	// updateCart(ctx, ord)
	// updateStats(ctx, org, ord, payments)
	// updateMailchimp(ctx, org, ord)
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

func saveReferral(org *organization.Organization, ord *order.Order) {
	ctx := org.Context()
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

func updateMailchimp(ctx appengine.Context, org *organization.Organization, ord *order.Order, usr *user.User) {
	// Save user as customer in Mailchimp if configured
	if org.Mailchimp.APIKey != "" {
		// Create new mailchimp client
		client := mailchimp.New(ctx, org.Mailchimp.APIKey)

		// Update cart
		car := cart.New(ord.Db)

		// Determine store to use
		storeId := ord.StoreId
		if storeId == "" {
			storeId = org.DefaultStore
		}

		if ord.CartId != "" {
			if err := car.GetById(ord.CartId); err != nil {
				log.Warn("Unable to find cart: %v", err, ctx)
			} else {
				// Delete cart in mailchimp
				if err := client.DeleteCart(storeId, car); err != nil {
					log.Warn("Failed to delete Mailchimp cart: %v", err, ctx)
				}
			}
		}

		stor := store.New(ord.Db)
		stor.MustGetById(storeId)

		// Subscribe user to list
		buy := Buyer{
			Email:     usr.Email,
			FirstName: usr.FirstName,
			LastName:  usr.LastName,
			Phone:     usr.Phone,
			Address:   ord.ShippingAddress,
		}

		referralLink := ""

		if err := usr.LoadReferrals(); err != nil {
			log.Warn("Failed to load referrals for user: %v", err, ctx)
		}

		if len(usr.Referrers) > 0 {
			referralLink = stor.ReferralBaseUrl + usr.Referrers[0].Id()
		}

		if err := client.SubscribeCustomer(stor.Mailchimp.ListId, buy, referralLink); err != nil {
			log.Warn("Failed to create Mailchimp order: %v", err, ctx)
		}

		// Create order in mailchimp
		if err := client.CreateOrder(storeId, ord); err != nil {
			log.Warn("Failed to create Mailchimp order: %v", err, ctx)
		}
	}
}
