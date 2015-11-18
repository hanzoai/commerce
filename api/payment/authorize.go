package payment

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/api/payment/balance"
	"crowdstart.com/api/payment/stripe"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/client"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/redis"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
)

func authorizationRequest(c *gin.Context, ord *order.Order) (*AuthorizationReq, error) {
	// Create AuthReq properly by calling order.New
	ar := new(AuthorizationReq)
	ar.Order = ord

	// Try decode request body
	if err := json.Decode(c.Request.Body, &ar); err != nil {
		log.Error("Failed to decode request body: %v\n%v", c.Request.Body, err, c)
		return nil, FailedToDecodeRequestBody
	}

	// This is kind of terrible to do here but oh well...
	if ar.Order.ShippingAddress.Empty() {
		ar.Order.ShippingAddress = ar.User_.ShippingAddress
	}

	if ar.Order.BillingAddress.Empty() {
		ar.Order.BillingAddress = ar.User_.BillingAddress
	}

	return ar, nil
}

func authorize(c *gin.Context, org *organization.Organization, ord *order.Order) (*payment.Payment, *user.User, error) {
	// Process authorization request
	ar, err := authorizationRequest(c, ord)
	if err != nil {
		return nil, nil, err
	}

	log.Debug("AuthorizationReq.User_: %#v", ar.User_, c)
	log.Debug("AuthorizationReq.Order: %#v", ar.Order, c)
	log.Debug("AuthorizationReq.Payment_: %#v", ar.Payment_, c)

	// Peel off order for convience
	ord = ar.Order
	ctx := ord.Db.Context

	// Check if store has been set, if so pull it out of the context
	var stor *store.Store
	v, ok := c.Get("store")
	if ok {
		stor = v.(*store.Store)
		ord.Currency = stor.Currency // Set currency
	}

	// Update order with information from datastore, store and tally
	if err := ord.UpdateAndTally(stor); err != nil {
		log.Error(err, ctx)
		return nil, nil, errors.New("Invalid or incomplete order")
	}

	log.Debug("Order: %#v", ord, c)

	// Get user from request
	usr, err := ar.User()
	if err != nil {
		return nil, nil, err
	}

	log.Debug("User: %#v", usr, c)

	// Get payment from request, update order
	pay, err := ar.Payment()
	if err != nil {
		return nil, nil, err
	}

	// Override total to $0.50 is test email is used
	if org.IsTestEmail(pay.Buyer.Email) {
		ord.Total = currency.Cents(50)
		pay.Test = true
	}

	// Use user as buyer
	pay.Buyer = usr.Buyer()
	log.Debug("Buyer: %#v", pay.Buyer, c)

	// Fill with debug information about user's browser
	pay.Client = client.New(c)

	// Update payment with order information
	pay.Amount = ord.Total

	// Fee defaults to 2%, override with organization fee if customized.
	pay.Fee = ord.CalculateFee(org.Fee)

	pay.Currency = ord.Currency
	pay.Description = ord.Description()

	log.Debug("Payment: %#v", pay, c)

	// Setup all relationships before we try to authorize to ensure that keys
	// that get created are actually valid.

	// User -> order
	ord.Parent = usr.Key()
	ord.UserId = usr.Id()

	// Order -> payment
	pay.Parent = ord.Key()
	pay.OrderId = ord.Id()

	// Save payment Id on order
	ord.PaymentIds = append(ord.PaymentIds, pay.Id())

	// Have stripe handle authorization
	switch ord.Type {
	case "paypal":
	case "balance":
		if err := balance.Authorize(org, ord, usr, pay); err != nil {
			log.Info("Failed to authorize order using Balance:\n User: %+v, Order: %+v, Payment: %+v, Error: %v", usr, ord, pay, err, ctx)
			return nil, nil, err
		}
	default:
		if err := stripe.Authorize(org, ord, usr, pay); err != nil {
			log.Info("Failed to authorize order using Stripe:\n User: %+v, Order: %+v, Payment: %+v, Error: %v", usr, ord, pay, err, ctx)
			return nil, nil, err
		}
	}

	// If the charge is not live or test flag is set, then it is a test charge
	ord.Test = pay.Test || !pay.Live

	ord.BillingAddress.Country = strings.ToUpper(ord.BillingAddress.Country)
	ord.ShippingAddress.Country = strings.ToUpper(ord.ShippingAddress.Country)

	if !ord.Test {
		if err := redis.IncrUsers(ctx, org, time.Now()); err != nil {
			log.Warn("Redis Error %s", err, ctx)
		}
	}

	// Save user, order, payment
	usr.MustPut()
	ord.MustPut()
	pay.MustPut()

	log.Info("New authorization for order\n User: %+v, Order: %+v, Payment: %+v", usr, ord, pay, ctx)

	return pay, usr, nil
}
