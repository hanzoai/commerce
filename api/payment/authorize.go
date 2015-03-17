package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/thirdparty/stripe2"

	"crowdstart.io/models2"
	"crowdstart.io/models2/order"
	"crowdstart.io/util/json"
)

type SourceType string

const (
	SourceCard   SourceType = "card"
	SourcePayPal            = "paypal"
	SourceAffirm            = "affirm"
)

type Buyer struct {
	Type SourceType `json:"type"`

	// Buyer
	Email     string         `json:"email"`
	FirstName string         `json:"firstName"`
	LastName  string         `json:"firstName"`
	Company   string         `json:"company"`
	Address   models.Address `json:"address"`
	Notes     string         `json:"notes"`

	// Card
	Number string `json:"number"`
	Month  string `json:"month"`
	CVC    string `json:"cvc"`
	Phone  string `json:"phone"`
	Year   string `json:"year"`
}

func (b Buyer) Card() *stripe.CardParams {
	card := stripe.CardParams{}
	card.Name = b.FirstName + " " + b.LastName
	card.Number = b.Number
	card.Month = b.Month
	card.Year = b.Year
	card.CVC = b.CVC
	card.Address1 = b.Address.Line1
	card.Address2 = b.Address.Line2
	card.City = b.Address.City
	card.State = b.Address.State
	card.Zip = b.Address.PostalCode
	card.Country = b.Address.Country
	return &card
}

func (b Buyer) Buyer() models.Buyer {
	buyer := models.Buyer{}
	buyer.FirstName = b.FirstName
	buyer.LastName = b.LastName
	buyer.Email = b.Email
	buyer.Phone = b.Phone
	buyer.Company = b.Company
	buyer.Notes = b.Notes
	return buyer
}

type AuthReq struct {
	Buyer Buyer        `json:"buyer"`
	Order *order.Order `json:"order"`
}

func authorize(c *gin.Context) (*order.Order, error) {
	// Get organization for this user
	org := middleware.GetOrg(c)
	ctx := org.Namespace(c)

	// Set up the db with the namespaced appengine context
	db := datastore.New(ctx)

	// Create AuthReq properly by calling order.New
	var ar AuthReq
	ar.Order = order.New(db)

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
	client := stripe.New(ctx, org.Stripe.PublishableKey)

	// Do authorization
	token, err := client.Authorize(ar.Buyer.Card())
	if err != nil {
		return nil, AuthorizationFailed
	}

	// Create account
	account := models.PaymentAccount{}
	account.Buyer = ar.Buyer.Buyer()
	account.Type = "stripe"

	// Create Stripe customer, which we will attach to our payment account.
	customer, err := client.NewCustomer(token.ID, account.Buyer)
	if err != nil {
		return nil, FailedToCreateCustomer
	}
	account.Stripe.CustomerId = customer.ID

	payment := models.Payment{}
	payment.Status = "unpaid"
	payment.Account = account
	payment.Amount = ar.Order.Total
	payment.Currency = ar.Order.Currency

	// Fill with debug information about user's browser
	// payment.Client =

	// Create charge and associate with payment.
	charge, err := client.NewCharge(customer, ar.Order.Total, ar.Order.Currency)
	payment.ChargeId = charge.ID

	// Create payment
	ar.Order.Payments = append(ar.Order.Payments, payment)

	return ar.Order, nil
}
