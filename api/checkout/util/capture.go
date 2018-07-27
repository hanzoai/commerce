package util

import (
	"context"

	"hanzo.io/log"
	"hanzo.io/models/cart"
	"hanzo.io/models/multi"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/referral"
	"hanzo.io/models/referrer"
	"hanzo.io/models/store"
	"hanzo.io/models/transaction"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/mailchimp"
	"hanzo.io/util/counter"

	. "hanzo.io/models"
)

func UpdateOrder(ctx context.Context, ord *order.Order, payments []*payment.Payment) {
	totalPaid := 0

	for _, pay := range payments {
		totalPaid += int(pay.Amount)
	}

	ord.Paid = currency.Cents(int(ord.Paid) + totalPaid)
	if ord.Paid >= ord.Total {
		ord.PaymentStatus = payment.Paid
	}

	ord.MustUpdate()
}

func UpdateOrderPayments(ctx context.Context, ord *order.Order, payments []*payment.Payment) error {
	vals := []interface{}{}

	for _, pay := range payments {
		vals = append(vals, pay)
	}

	return multi.Update(vals)
}

func SaveRedemptions(ctx context.Context, ord *order.Order) {
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

type Referrent struct {
	id string
	kind string
	total currency.Cents
}

func (r *Referrent) Id() string {
	return r.id
}

func (r *Referrent) Kind() string {
	return r.kind
}

func (r *Referrent) Total() currency.Cents {
	return r.total
}

func UpdateReferral(org *organization.Organization, ord *order.Order) {
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

	// Total the order
	total := ord.Total
	for _, sub := range(ord.Subscriptions) {
		total += sub.Total
	}

	// Save referral
	rfl, err := ref.SaveReferral(ctx, org.Id(), referral.NewOrder, &Referrent{
		ord.Id(),
		ord.Kind(),
		total,
	}, !org.Live)

	if err != nil {
		log.Warn("Unable to save referral: %v", err, ctx)
		return
	}

	if !ord.Test {
		if err := counter.IncrReferrerFees(ctx, org, ref.Id(), rfl); err != nil {
			log.Warn("Counter Error %s", err, ctx)
		}
	}

	// Update statistics
	if ref.AffiliateId != "" {
		if err := counter.IncrAffiliateFees(ctx, org, ref.AffiliateId, rfl); err != nil {
			log.Warn("Counter Error %s", err, ctx)
		}
	}
}

func UpdateCart(ctx context.Context, ord *order.Order) {
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

func UpdateStats(ctx context.Context, org *organization.Organization, ord *order.Order, payments []*payment.Payment) {
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

		if err := counter.IncrOrder(ctx, ord); err != nil {
			log.Error("IncrOrder Error %v", err, ctx)
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

func UpdateMailchimp(ctx context.Context, org *organization.Organization, ord *order.Order, usr *user.User) {
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
			BillingAddress:   ord.BillingAddress,
			ShippingAddress:  ord.ShippingAddress,
		}

		referralLink := ""

		referrers := make([]referrer.Referrer, 0)
		if _, err := referrer.Query(ord.Db).Filter("UserId=", usr.Id()).GetAll(&referrers); err != nil {
			log.Warn("Failed to load referrals for user: %v", err, ctx)
		}

		if len(referrers) > 0 {
			referralLink = stor.ReferralBaseUrl + referrers[0].Id_
		}

		log.Warn("Referral Link: %v from %v", referralLink, usr.Referrers, ctx)

		if err := client.SubscribeCustomer(stor.Mailchimp.ListId, buy, referralLink); err != nil {
			log.Warn("Failed to create Mailchimp customer - status: %v", err.Status, ctx)
			log.Warn("Failed to create Mailchimp customer - unknown error: %v", err.Unknown, ctx)
			log.Warn("Failed to create Mailchimp customer - mailchimp error: %v", err.Mailchimp, ctx)
		}

		// Create order in mailchimp
		if err := client.CreateOrder(storeId, ord); err != nil {
			log.Warn("Failed to create Mailchimp order - status: %v", err.Status, ctx)
			log.Warn("Failed to create Mailchimp order - unknown error: %v", err.Unknown, ctx)
			log.Warn("Failed to create Mailchimp order - mailchimp error: %v", err.Mailchimp, ctx)
		}
	}
}

func HandleDeposit(ord *order.Order) {
	// Handle Deposit Logic
	if ord.Mode == order.DepositMode && ord.PaymentStatus == payment.Paid {
		trans := transaction.New(ord.Db)
		trans.DestinationId = ord.UserId
		trans.DestinationKind = "user"
		trans.Type = transaction.Deposit
		trans.Currency = ord.Currency
		trans.Amount = ord.Subtotal
		trans.Test = ord.Test
		trans.Notes = "Deposit from Order '" + ord.Id() + "'"
		trans.MustCreate()
	}
}
