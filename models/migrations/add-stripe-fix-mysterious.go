package migrations

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"hanzo.io/models"
	"hanzo.io/models/lineitem"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/stripe"
	"hanzo.io/log"

	ds "hanzo.io/datastore"
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
			usr.History = []models.Event{models.Event{"RegeneratedFromStripe", "Mysteriously Missing 2015-07-02"}}
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
			ord.History = []models.Event{models.Event{"RegeneratedFromStripe", "Mysteriously Missing 2015-07-02"}}

			descs := strings.Split(charge.Desc, ",")
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
	},
)
