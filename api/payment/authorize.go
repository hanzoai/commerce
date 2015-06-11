package payment

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.com/api/payment/balance"
	"crowdstart.com/api/payment/stripe"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
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

	// Peel off order for convience
	ord = ar.Order
	ctx := ord.Db.Context

	// Check if store has been set, if so pull it out of the context
	var stor *store.Store
	v, err := c.Get("store")
	if err == nil {
		stor = v.(*store.Store)
		ord.Currency = stor.Currency // Set currency
	}

	// Update order with information from datastore, store and tally
	if err := ord.UpdateAndTally(stor); err != nil {
		log.Error(err, ctx)
		return nil, nil, errors.New("Invalid or incomplete order")
	}

	log.Debug("Order: %#v", ord)

	// Get user from request
	usr, err := ar.User()
	if err != nil {
		return nil, nil, err
	}

	log.Debug("User: %#v", usr)

	// Get payment from request, update order
	pay, err := ar.Payment()
	if err != nil {
		return nil, nil, err
	}

	// Use user as buyer
	pay.Buyer = usr.Buyer()

	// Fill with debug information about user's browser
	pay.Client.Ip = c.Request.RemoteAddr
	pay.Client.UserAgent = c.Request.UserAgent()
	pay.Client.Language = c.Request.Header.Get("Accept-Language")
	pay.Client.Referer = c.Request.Referer()

	// Update payment with order information
	pay.Amount = ord.Total
	pay.Fee = ord.Fee()
	pay.Currency = ord.Currency
	pay.Description = ord.Description()

	log.Debug("Payment: %#v", pay)

	// Set order total to $0.50 if using a test email
	if org.IsTestEmail(pay.Buyer.Email) {
		pay.Amount = currency.Cents(50)
		ord.Total = currency.Cents(50)
		pay.Test = true
	}

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
	case "balance":
		if err := balance.Authorize(org, ord, usr, pay); err != nil {
			return nil, nil, err
		}
	default:
		if err := stripe.Authorize(org, ord, usr, pay); err != nil {
			return nil, nil, err
		}
	}

	// If the charge is not live or test flag is set, then it is a test charge
	ord.Test = pay.Test || !pay.Live

	ord.BillingAddress.Country = strings.ToUpper(ord.BillingAddress.Country)
	ord.ShippingAddress.Country = strings.ToUpper(ord.ShippingAddress.Country)

	// Save user, order, payment
	usr.MustPut()
	ord.MustPut()
	pay.MustPut()

	return pay, usr, nil
}
