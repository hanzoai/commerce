package checkout

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/api/checkout/stripe"
	"hanzo.io/log"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/product"
	"hanzo.io/models/referral"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/util/counter"
	"hanzo.io/util/json"

	"hanzo.io/thirdparty/woopra"
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

	if err := stripe.Refund(org, ord, req.Amount); err != nil {
		return err
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

			values := make(map[string]string)
			values["firstName"] = usr.FirstName
			values["lastName"] = usr.LastName
			values["city"] = usr.ShippingAddress.City
			values["country"] = usr.ShippingAddress.Country
			values["referred_by"] = usr.ReferrerId
			values["referrals"] = strconv.Itoa(len(usr.Referrals))

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
				"removeReferral", // event name
				map[string]string{ // custom data
					"referredOrderId":     ord.Id(),
					"referredOrderNumber": strconv.Itoa(ord.Number),
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
		}
	}

	return nil
}
