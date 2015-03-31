package stripe

import (
	"strconv"
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"

	"crowdstart.io/models2/payment"
	"crowdstart.io/models2/user"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
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

// Covert a payment model into a card card we can use for authorization
func PaymentToCard(pay *payment.Payment) *stripe.CardParams {
	card := stripe.CardParams{}
	card.Name = pay.Buyer.Name()
	card.Number = pay.Account.Number
	card.CVC = pay.Account.CVC
	card.Month = strconv.Itoa(pay.Account.Month)
	card.Year = strconv.Itoa(pay.Account.Year)
	card.Address1 = pay.Buyer.Address.Line1
	card.Address2 = pay.Buyer.Address.Line2
	card.City = pay.Buyer.Address.City
	card.State = pay.Buyer.Address.State
	card.Zip = pay.Buyer.Address.PostalCode
	card.Country = pay.Buyer.Address.Country
	return &card
}

// Do authorization, return token
func (c Client) Authorize(pay *payment.Payment) (*Token, error) {
	t, err := c.API.Tokens.New(&stripe.TokenParams{
		Card: PaymentToCard(pay),
	})

	if err != nil {
		return nil, NewError(err)
	}

	// Cast back to our token
	return (*Token)(t), err
}

// Get an exising Stripe card
func (c Client) GetCard(cardId string, customerId string) (*Card, error) {
	params := &stripe.CardParams{
		Customer: customerId,
	}

	card, err := c.API.Cards.Get(cardId, params)
	if err != nil {
		return nil, NewError(err)
	}

	return (*Card)(card), err
}

// Get Stripe customer
func (c Client) GetCustomer(token, user *user.User) (*Customer, error) {
	params := &stripe.CustomerParams{}
	params.SetSource(token)

	customerId := user.Accounts.Stripe.CustomerId

	customer, err := c.API.Customers.Get(customerId, params)
	if err != nil {
		return nil, NewError(err)
	}

	return (*Customer)(customer), err
}

// Update Stripe customer
func (c Client) UpdateCustomer(user *user.User) (*Customer, error) {
	params := &stripe.CustomerParams{
		Email: user.Email,
	}

	// Update with our user metadata
	for k, v := range user.Metadata {
		params.AddMeta(k, json.Encode(v))
	}

	params.AddMeta("user", user.Id())

	customerId := user.Accounts.Stripe.CustomerId

	customer, err := c.API.Customers.Update(customerId, params)
	if err != nil {
		return nil, NewError(err)
	}

	return (*Customer)(customer), err
}

// Create new stripe customer
func (c Client) NewCustomer(token string, user *user.User) (*Customer, error) {
	params := &stripe.CustomerParams{
		Desc:  user.Name(),
		Email: user.Email,
	}
	params.SetSource(token)

	// Update with our user metadata
	for k, v := range user.Metadata {
		params.AddMeta(k, json.Encode(v))
	}

	params.AddMeta("user", user.Id())

	customer, err := c.API.Customers.New(params)
	if err != nil {
		return nil, NewError(err)
	}

	return (*Customer)(customer), err
}

// Add new card to Stripe customer
func (c Client) AddCard(token string, user *user.User) (*Card, error) {
	params := &stripe.CardParams{
		Customer: user.Accounts.Stripe.CustomerId,
		Token:    token,
	}

	card, err := c.API.Cards.New(params)
	if err != nil {
		return nil, NewError(err)
	}

	return (*Card)(card), err
}

// Update card associated with Stripe customer
func (c Client) UpdateCard(token string, pay *payment.Payment, user *user.User) (*Card, error) {
	acct := user.Accounts.Stripe
	customerId := acct.CustomerId
	cardId := acct.CardId

	params := &stripe.CardParams{
		Customer: customerId,
		Token:    token,
	}

	card, err := c.API.Cards.Update(cardId, params)
	if err != nil {
		return nil, NewError(err)
	}

	return (*Card)(card), err
}

// Create new charge
func (c Client) NewCharge(source interface{}, pay *payment.Payment) (*Charge, error) {
	params := &stripe.ChargeParams{
		Amount:    uint64(pay.Amount),
		Fee:       uint64(pay.Fee),
		Currency:  stripe.Currency(pay.Currency),
		Customer:  pay.Account.CustomerId,
		Email:     pay.Buyer.Email,
		Desc:      pay.Description,
		NoCapture: true,
	}

	// Update with our user metadata
	for k, v := range pay.Metadata {
		params.AddMeta(k, json.Encode(v))
	}

	params.AddMeta("payment", pay.Id())

	switch v := source.(type) {
	case string:
		params.SetSource(v)
	case *Customer:
		params.Customer = v.ID
	case *user.User:
		params.Customer = v.Accounts.Stripe.CustomerId
		params.AddMeta("user", v.Id())
	}

	// Create charge
	ch, err := c.API.Charges.New(params)
	if err != nil {
		return nil, NewError(err)
	}

	// Set Charge Id
	pay.ChargeId = ch.ID

	return (*Charge)(ch), err
}

// Capture charge
func (c Client) Capture(id string) (*Charge, error) {
	log.Warn("Capture %v", id)
	ch, err := c.API.Charges.Capture(id, nil)
	if err != nil {
		return nil, NewError(err)
	}

	return (*Charge)(ch), err
}
