package stripe

import (
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/token"

	"crowdstart.io/models2"
)

type Card stripe.CardParams
type Charge stripe.Charge
type Customer stripe.Customer
type Token stripe.Token

type Client struct {
	*client.API
	ctx appengine.Context
}

func New(ctx appengine.Context, publishableKey string) *Client {
	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(ctx)
	httpClient.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(10) * time.Second, // Update deadline to 10 seconds
	}
	stripe.SetHTTPClient(httpClient)

	sc := &client.API{}
	sc.Init(publishableKey, nil)
	return &Client{sc, ctx}
}

// Do authorization, return token
func (c Client) Authorize(card *Card) (*Token, error) {
	// Create a new token
	t, err := token.New(&stripe.TokenParams{
		Card: (*stripe.CardParams)(card),
	})

	// Cast back to our token
	return (*Token)(t), err
}

// Create new stripe customer
func (c Client) NewCustomer(token string, card *Card, buyer *models.Buyer) (*Customer, error) {
	params := &stripe.CustomerParams{
		Desc:  buyer.Name(),
		Email: buyer.Email,
	}
	params.SetSource(token)

	customer, err := c.API.Customers.New(params)

	return (*Customer)(customer), err
}

// Create new charge
func (c Client) NewCharge(customerOrToken interface{}, amount models.Cents, currency models.CurrencyType) (*Charge, error) {
	chargeParams := &stripe.ChargeParams{
		Amount: uint64(amount),
		// Currency: string(currency),
		Desc: "Charge for test@example.com",
	}

	switch v := customerOrToken.(type) {
	case string:
		chargeParams.SetSource(v)
	case *Customer:
		chargeParams.Customer = v.ID
	}

	// Create charge
	ch, err := charge.New(chargeParams)

	return (*Charge)(ch), err
}
