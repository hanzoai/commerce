package checkout

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/api/checkout/square"
	"github.com/hanzoai/commerce/api/checkout/stripe"
	"github.com/hanzoai/commerce/models/types/accounts"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/referral"
	"github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/types/integration"
	"github.com/hanzoai/commerce/util/counter"
	"github.com/hanzoai/commerce/util/json"

	"github.com/hanzoai/commerce/thirdparty/mailchimp"
	"github.com/hanzoai/commerce/thirdparty/woopra"
	. "github.com/hanzoai/commerce/types"
)

func refund(c *gin.Context, org *organization.Organization, ord *order.Order) error {
	type Refund struct {
		Amount currency.Cents `json:"amount"`
	}

	req := new(Refund)
	if err := json.Decode(c.Request.Body, req); err != nil {
		log.Error("Failed to decode request body: %v\n%v", c.Request.Body, err, c)
		return FailedToDecodeRequestBody
	}

	log.JSON(req)

	switch ord.Type {
	case accounts.SquareType:
		if err := square.Refund(org, ord, req.Amount); err != nil {
			return err
		}
	case accounts.StripeType:
		if err := stripe.Refund(org, ord, req.Amount); err != nil {
			return err
		}
	default:
		// Default to Square
		if err := square.Refund(org, ord, req.Amount); err != nil {
			return err
		}
	}

	ctx := ord.Context()

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

		in := org.Integrations.FindByType(integration.WoopraType)
		if in != nil {
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
	}

	if !ord.Test {
		if err := counter.IncrOrderRefund(ord.Context(), ord, int(req.Amount), time.Now()); err != nil {
			log.Error("IncrOrderRefund Error %v", err, c)
		}

		if ord.Total == ord.Refunded {
			if err := ord.GetItemEntities(); err != nil {
				for _, item := range ord.Items {
					prod := product.New(ord.Db)

					if err := prod.GetById(item.ProductId); err != nil {
						log.Error("no product found %v", err, c)
					}

					counter.IncrProductRefund(ord.Context(), prod, ord)
				}
			}

			if org.Mailchimp.APIKey != "" {
				// Create new mailchimp client
				client := mailchimp.New(ctx, org.Mailchimp)

				usr := user.New(ord.Db)
				if err := usr.GetById(ord.UserId); err != nil {
					log.Warn("Could not get order's user '%s'", ord.UserId, ctx)
					return err
				}

				if err := usr.LoadOrders(); err != nil {
					log.Error("loadorders error %v", err, c)
					return nil
				}

				paidOrders := 0
				for _, v := range usr.Orders {
					switch ps := v.PaymentStatus; ps {
					case payment.Paid:
						if !v.Test {
							paidOrders++
						}
					}
				}

				if paidOrders == 0 {
					// Determine store to use
					storeId := ord.StoreId
					if storeId == "" {
						storeId = org.DefaultStore
					}

					stor := store.New(ord.Db)
					stor.MustGetById(storeId)

					// Subscribe user to list
					buy := Buyer{
						Email:           usr.Email,
						FirstName:       usr.FirstName,
						LastName:        usr.LastName,
						Phone:           usr.Phone,
						BillingAddress:  ord.BillingAddress,
						ShippingAddress: ord.ShippingAddress,
					}

					if err := client.UnsubscribeCustomer(stor.Mailchimp.ListId, buy); err != nil {
						log.Warn("Failed to delete Mailchimp customer - status: %v", err.Status, ctx)
						log.Warn("Failed to delete Mailchimp customer - unknown error: %v", err.Unknown, ctx)
						log.Warn("Failed to delete Mailchimp customer - mailchimp error: %v", err.Mailchimp, ctx)
					}
				}
			}
		}
	}

	in := org.Integrations.FindByType(integration.WoopraType)
	if in != nil {
		usr := user.New(ord.Db)
		if err := usr.GetById(ord.UserId); err != nil {
			log.Error("no user found %v", err, c)
			return nil
		}

		if err := usr.LoadOrders(); err != nil {
			log.Error("loadorders error %v", err, c)
			return nil
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

		cancelledOrders := 0
		creditOrders := 0
		disputedOrders := 0
		failedOrders := 0
		fraudOrders := 0
		paidOrders := 0
		refundedOrders := 0
		unpaidOrders := 0
		for _, v := range usr.Orders {
			switch ps := v.PaymentStatus; ps {
			case payment.Cancelled:
				cancelledOrders++
			case payment.Credit:
				creditOrders++
			case payment.Disputed:
				disputedOrders++
			case payment.Failed:
				failedOrders++
			case payment.Fraudulent:
				fraudOrders++
			case payment.Paid:
				paidOrders++
			case payment.Refunded:
				refundedOrders++
			case payment.Unpaid:
				unpaidOrders++
			}
		}

		values := make(map[string]string)
		values["first_name"] = usr.FirstName
		values["last_name"] = usr.LastName
		values["city"] = usr.ShippingAddress.City
		values["country"] = usr.ShippingAddress.Country
		values["referred_by"] = usr.ReferrerId
		values["referrals"] = strconv.Itoa(len(usr.Referrals))
		values["orders"] = strconv.Itoa(len(usr.Orders))
		values["cancelled_orders"] = strconv.Itoa(cancelledOrders)
		values["credit_orders"] = strconv.Itoa(creditOrders)
		values["disputed_orders"] = strconv.Itoa(disputedOrders)
		values["failed_orders"] = strconv.Itoa(failedOrders)
		values["fraud_orders"] = strconv.Itoa(fraudOrders)
		values["paid_orders"] = strconv.Itoa(paidOrders)
		values["refunded_orders"] = strconv.Itoa(refundedOrders)
		values["unpaid_orders"] = strconv.Itoa(unpaidOrders)

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

	return nil
}
