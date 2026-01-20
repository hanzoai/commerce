package migrations

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/lineitem"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/thirdparty/stripe"

	ds "github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var _ = New("add-stripe-fix-mysterious",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")

		db := ds.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("Name=", "bellabeat").Get(); err != nil {
			panic(err)
		}
		return []interface{}{org.Stripe.AccessToken}
	},
	func(db *ds.Datastore, pay *payment.Payment, accessToken string) {
		if !pay.Live || pay.Test {
			log.Debug("Test Payment Encountered", db.Context)
			return
		}

		usr := user.New(db)
		if err := usr.GetByEmail(pay.Buyer.Email); err != nil {
			buyer := pay.Buyer

			usr.Email = buyer.Email
			usr.FirstName = buyer.FirstName
			usr.LastName = buyer.LastName
			usr.Company = buyer.Company
			usr.Phone = buyer.Phone
			usr.ShippingAddress = buyer.ShippingAddress

			usr.History = []Event{{Type: "RegeneratedFromStripe", Desc: "Mysteriously Missing 2015-07-02"}}
			usr.Accounts.Stripe = pay.Account.Stripe

			usr.MustPut()
		}

		ord := order.New(db)
		if err := ord.GetById(pay.OrderId); err != nil {
			sc := stripe.New(db.Context, accessToken)
			charge, err := sc.GetCharge(pay.Account.ChargeId)
			if err != nil {
				log.Error("Stripe error encoutnered %v", err, db.Context)
				return
			}

			log.Debug("Order Is Missing", db.Context)
			ord.ShippingAddress = pay.Buyer.ShippingAddress
			ord.Subtotal = pay.Amount
			ord.Total = pay.Amount
			ord.Currency = pay.Currency

			ord.History = []Event{{Type: "RegeneratedFromStripe", Desc: "Mysteriously Missing 2015-07-02"}}

			descs := strings.Split(charge.Description, ",")
			ord.Items = make([]lineitem.LineItem, len(descs))

			for i, desc := range descs {
				tokens := strings.Split(desc, "x")
				if len(tokens) != 2 {
					log.Warn("Malformed description detected", db.Context)
				}

				val, err := strconv.Atoi(strings.TrimSpace(tokens[1]))
				if err != nil {
					log.Warn("Malformed description detected, quanity not an int %v", err, db.Context)
				}

				ord.Items[i] = lineitem.LineItem{
					ProductName: strings.TrimSpace(tokens[0]),
					Quantity:    val,
				}
			}

			if charge.Refunded {
				ord.Status = order.Cancelled
				ord.PaymentStatus = payment.Refunded
			}
			ord.UserId = usr.Id()
			ord.MustPut()
		}

		pay.Buyer.UserId = usr.Id()
		pay.OrderId = ord.Id()
		pay.MustPut()
		log.Debug("Updating Payment %v, Order %v, UserId %v", pay.Id(), ord.Id(), usr.Id(), db.Context)

	})
