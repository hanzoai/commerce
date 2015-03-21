package stripe

import (
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"

	"crowdstart.io/models2"
)

type Card stripe.Card
type CardParams stripe.CardParams
type Charge stripe.Charge
type Customer stripe.Customer
type Token stripe.Token

type Client struct {
	*client.API
	ctx appengine.Context
}

func New(ctx appengine.Context, accessToken string) *Client {
	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(ctx)
	httpClient.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(10) * time.Second, // Update deadline to 10 seconds
	}
	stripe.SetHTTPClient(httpClient)

	sc := &client.API{}
	sc.Init(accessToken, nil)
	return &Client{sc, ctx}
}

// Do authorization, return token
func (c Client) Authorize(card *CardParams) (*Token, error) {
	t, err := c.API.Tokens.New(&stripe.TokenParams{
		Card: (*stripe.CardParams)(card),
	})

	if err != nil {
		stripeErr, ok := err.(*stripe.Error)
		if ok {
			return nil, &Error{
				Code:    string(stripeErr.Code),
				Message: stripeErr.Msg,
				Type:    string(stripeErr.Type),
			}
		}
		return nil, &Error{Type: "unknown", Message: "Stripe: authorization failed."}
	}

	// Cast back to our token
	return (*Token)(t), err
}

// Create new stripe customer
func (c Client) GetCard(cardId string, customerId string) (*Card, error) {
	params := &stripe.CardParams{
		Customer: customerId,
	}

	card, err := c.API.Cards.Get(cardId, params)
	if err != nil {
		stripeErr, ok := err.(*stripe.Error)
		if ok {
			return nil, &Error{
				Code:    string(stripeErr.Code),
				Message: stripeErr.Msg,
				Type:    string(stripeErr.Type),
			}
		}
		return nil, &Error{Type: "unknown", Message: "Stripe: failed to get card"}
	}

	return (*Card)(card), err
}

// Create new stripe customer
func (c Client) NewCustomer(token string, buyer models.Buyer) (*Customer, error) {
	params := &stripe.CustomerParams{
		Desc:  buyer.Name(),
		Email: buyer.Email,
	}
	params.SetSource(token)

	customer, err := c.API.Customers.New(params)
	if err != nil {
		stripeErr, ok := err.(*stripe.Error)
		if ok {
			return nil, &Error{
				Code:    string(stripeErr.Code),
				Message: stripeErr.Msg,
				Type:    string(stripeErr.Type),
			}
		}
		return nil, &Error{Type: "unknown", Message: "Stripe: failed to create customer"}
	}

	return (*Customer)(customer), err
}

// Create new charge
func (c Client) NewCharge(customerOrToken interface{}, payment models.Payment) (*Charge, error) {
	chargeParams := &stripe.ChargeParams{
		Amount:    uint64(payment.Amount),
		Currency:  stripe.Currency(payment.Currency),
		Desc:      "Charge for test@example.com",
		NoCapture: true,
	}

	switch v := customerOrToken.(type) {
	case string:
		chargeParams.SetSource(v)
	case *Customer:
		chargeParams.Customer = v.ID
	}

	// Create charge
	ch, err := c.API.Charges.New(chargeParams)
	if err != nil {
		stripeErr, ok := err.(*stripe.Error)
		if ok {
			return nil, &Error{
				Code:    string(stripeErr.Code),
				Message: stripeErr.Msg,
				Type:    string(stripeErr.Type),
			}
		}
		return nil, &Error{Type: "unknown", Message: "Stripe: charge failed"}
	}

	return (*Charge)(ch), err
}

// Capture charge
func (c Client) Capture(id string) (*Charge, error) {
	ch, err := c.API.Charges.Capture(id, nil)
	if err != nil {
		stripeErr, ok := err.(*stripe.Error)
		if ok {
			return nil, &Error{
				Code:    string(stripeErr.Code),
				Message: stripeErr.Msg,
				Type:    string(stripeErr.Type),
			}
		}
		return nil, &Error{Type: "unknown", Message: "Stripe: capture failed"}
	}

	return (*Charge)(ch), err
}
