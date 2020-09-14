package tasks

import (
	"context"
	"strconv"
	"strings"
	"time"

	"hanzo.io/delay"

	"hanzo.io/datastore"
	"hanzo.io/log"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/referral"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/types/integration"

	"hanzo.io/thirdparty/woopra"
)

var updateOrder = delay.Func("stripe-update-order", func(ctx context.Context, ns string, orderId string, refunded currency.Cents, start time.Time) {
	db := datastore.New(ctx)
	org := organization.New(db)
	if err := org.GetById(ns); err != nil {
		log.Warn("Org cannot be found %v", org, ctx)
		return
	}

	nsctx := getNamespacedContext(ctx, ns)
	nsdb := datastore.New(nsctx)
	ord := order.New(nsdb)

	log.Debug("Updating order '%s'", orderId, ctx)

	if start.Before(ord.UpdatedAt) {
		log.Warn("Order has already been updated %v", ord, ctx)
		return
	}

	err := ord.RunInTransaction(func() error {
		err := ord.GetById(orderId)
		if err != nil {
			return err
		}

		// Update order using latest payment information
		log.Debug("Order before: %+v", ord, ctx)
		ord.UpdatePaymentStatus()
		if ord.Total == ord.Refunded && ord.ReferralId != "" {
			rfl := referral.New(ord.Db)
			if err := rfl.GetById(ord.ReferralId); err != nil {
				return err
			}
			rfl.Revoked = true
			if err := rfl.Update(); err != nil {
				return err
			}

			usr := user.New(ord.Db)
			if err := usr.GetById(rfl.Referrer.UserId); err != nil {
				log.Warn("Could not get referring user '%s'", rfl.Referrer.UserId, ctx)
				return err
			}

			if err := usr.LoadReferrals(); err != nil {
				log.Warn("Could not load referring user's referrals '%s'", rfl.Referrer.UserId, ctx)
				return err
			}

			domains := strings.Split(rfl.Referrer.WoopraDomains, ",")

			for _, domain := range domains {
				wt, _ := woopra.NewTracker(map[string]string{

					// `host` is domain as registered in Woopra, it identifies which
					// project environment to receive the tracking request
					"host": domain,

					// In milliseconds, defaults to 30000 (equivalent to 30 seconds)
					// after which the event will expire and the visit will be marked
					// as offline.
					"timeout": "30000",
				})

				revokedReferrals := 0
				for _, v := range usr.Referrals {
					if v.Revoked {
						revokedReferrals += 1
					}
				}

				values := make(map[string]string)
				values["first_name"] = usr.FirstName
				values["last_name"] = usr.LastName
				values["city"] = usr.ShippingAddress.City
				values["country"] = usr.ShippingAddress.Country
				values["referred_by"] = usr.ReferrerId
				values["referrals"] = strconv.Itoa(len(usr.Referrals))
				values["active_referrals"] = strconv.Itoa(len(usr.Referrals) - revokedReferrals)
				values["revoked_referrals"] = strconv.Itoa(revokedReferrals)

				person := woopra.Person{
					Id:     usr.Id(),
					Name:   usr.Name(),
					Email:  usr.Email,
					Values: values,
				}

				// identifying current visitor in Woopra
				ident := wt.Identify(
					ctx,
					person,
					"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/601.7.7 (KHTML, like Gecko) Version/9.1.2 Safari/601.7.7",
				)

				// Tracking custom event in Woopra. Each event can has additional data
				ident.Track(
					"Remove Referral", // event name
					map[string]string{ // custom data
						"referred_order_id":     ord.Id(),
						"referred_order_number": strconv.Itoa(ord.Number),
					})

				// it's possible to send only visitor's data to Woopra, without sending
				// any custom event and/or data
				ident.Push()
			}

			in := org.Integrations.FindByType(integration.WoopraType)
			if in != nil {
				usr := user.New(ord.Db)
				if err := usr.GetById(ord.UserId); err != nil {

				}
				domain := in.Woopra.Domain
				wt, _ := woopra.NewTracker(map[string]string{

					// `host` is domain as registered in Woopra, it identifies which
					// project environment to receive the tracking request
					"host": domain,

					// In milliseconds, defaults to 30000 (equivalent to 30 seconds)
					// after which the event will expire and the visit will be marked
					// as offline.
					"timeout": "30000",
				})

				values := make(map[string]string)
				values["first_name"] = usr.FirstName
				values["last_name"] = usr.LastName
				values["city"] = usr.ShippingAddress.City
				values["country"] = usr.ShippingAddress.Country
				values["referred_by"] = usr.ReferrerId

				person := woopra.Person{
					Id:     usr.Id(),
					Name:   usr.Name(),
					Email:  usr.Email,
					Values: values,
				}

				// identifying current visitor in Woopra
				ident := wt.Identify(
					ctx,
					person,
					"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/601.7.7 (KHTML, like Gecko) Version/9.1.2 Safari/601.7.7",
				)

				// Tracking custom event in Woopra. Each event can has additional data
				ident.Track(
					"Order Refund", // event name
					map[string]string{ // custom data
						"order_id":     ord.Id(),
						"order_number": strconv.Itoa(ord.Number),
						"name":         usr.Name(),
						"email":        usr.Email,
						"city":         usr.ShippingAddress.City,
						"country":      usr.ShippingAddress.Country,
						"referred_by":  usr.ReferrerId,
						"currency":     string(ord.Currency),
						"revenue":      ord.Currency.ToStringNoSymbol(ord.Total),
						"refunded":     ord.Currency.ToStringNoSymbol(ord.Refunded),
						"cart_id":      ord.CartId,
					})

				// it's possible to send only visitor's data to Woopra, without sending
				// any custom event and/or data
				ident.Push()
			}
		}

		log.Debug("Order after: %+v", ord, ctx)

		return ord.Put()
	}, nil)

	if err != nil {
		log.Error("Failed to update order '%s': %v", orderId, err, ctx)
	}
})
