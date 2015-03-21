package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/thirdparty/stripe2"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

func authorize(c *gin.Context, org *organization.Organization, ord *order.Order) (*order.Order, error) {
	// Get namespaced context off order
	ctx := ord.Db.Context

	// Create AuthReq properly by calling order.New
	ar := new(AuthReq)
	ar.Order = ord

	// In case people are using the version of the api that takes
	// existing orders Update order in request with id
	if id := c.Params.ByName("id"); id != "" {
		if err := ar.Order.Get(id); err != nil {
			return nil, OrderDoesNotExist
		}
	}

	// Try decode request body
	if err := json.Decode(c.Request.Body, &ar); err != nil {
		return nil, FailedToDecodeRequestBody
	}

	// Get client we can use for API calls
	client := stripe.New(ctx, org.Stripe.AccessToken)

	// Update order totals
	ar.Order.Tally()
	log.Debug("Order: %#v", ar.Order)

	// Do authorization
	token, err := client.Authorize(ar.Source.Card())
	if err != nil {
		return nil, err
	}

	// Create account
	account := models.PaymentAccount{}
	account.Buyer = ar.Source.Buyer()
	account.Type = "stripe"

	// Create Stripe customer, which we will attach to our payment account.
	customer, err := client.NewCustomer(token.ID, account.Buyer)
	if err != nil {
		return nil, err
	}
	account.Stripe.CustomerId = customer.ID

	log.Debug("Stripe customer: %#v", customer)
	log.Debug("Stripe source: %#v", customer.DefaultSource)

	// Get default source
	cardId := customer.DefaultSource.ID
	card, err := client.GetCard(cardId, customer.ID)
	if err != nil {
		return nil, err
	}

	account.Stripe.CardId = cardId
	account.Stripe.Brand = string(card.Brand)
	account.Stripe.LastFour = card.LastFour
	account.Stripe.Expiration.Month = int(card.Month)
	account.Stripe.Expiration.Year = int(card.Year)
	account.Stripe.Country = card.Country
	account.Stripe.Fingerprint = card.Fingerprint
	account.Stripe.Type = string(card.Funding)
	account.Stripe.CVCCheck = string(card.CVCCheck)

	payment := models.Payment{}
	payment.Status = "unpaid"
	payment.Account = account
	payment.Amount = ar.Order.Total
	payment.Currency = ar.Order.Currency

	// Fill with debug information about user's browser
	// payment.Client =

	// Create charge and associate with payment.
	charge, err := client.NewCharge(customer, payment)
	if err != nil {
		return nil, err
	}
	payment.ChargeId = charge.ID

	// Create payment
	ar.Order.Payments = append(ar.Order.Payments, payment)

	// Save order!
	ar.Order.Put()

	return ar.Order, nil
}
