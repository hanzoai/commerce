package stripe

import (
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"

	"crowdstart.com/models/payment"
	"crowdstart.com/models/subscription"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/stripe/errors"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
)

type Client struct {
	*client.API
	ctx appengine.Context
}

type Payable interface {
	ToCard() *stripe.CardParams
}

func New(ctx appengine.Context, accessToken string) *Client {
	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(ctx)
	httpClient.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(20) * time.Second, // Update deadline to 10 seconds
	}
	stripe.SetBackend(stripe.APIBackend, nil)
	stripe.SetHTTPClient(httpClient)

	sc := &client.API{}
	sc.Init(accessToken, nil)
	return &Client{sc, ctx}
}

// Covert a payment model into a card card we can use for authorization
func ToCard(pay Payable) *stripe.CardParams {
	card := pay.ToCard()
	return card
}

// Subscribe to a plan
func (c Client) NewSubscription(token string, source interface{}, sub *subscription.Subscription) (*Sub, error) {
	log.Warn("sub.Plan %v", sub.Plan)
	params := stripe.SubParams{
		Plan:  sub.Plan.StripeId,
		Token: token,
	}

	switch v := source.(type) {
	case *Customer:
		params.Customer = v.ID
	case *user.User:
		params.Customer = v.Accounts.Stripe.CustomerId
		params.AddMeta("user", v.Id())
	}

	params.AddMeta("plan", sub.Plan.Id())

	s, err := c.Subs.New(&params)
	if err != nil {
		return nil, errors.New(err)
	}

	sub.StripeId = s.ID
	sub.StripeCustomerId = params.Customer
	sub.FeePercent = s.FeePercent
	sub.EndCancel = s.EndCancel
	sub.PeriodStart = time.Unix(s.PeriodStart, 0)
	sub.PeriodEnd = time.Unix(s.PeriodEnd, 0)
	// sub.Start = time.Unix(s.Start, 0)
	sub.Ended = time.Unix(s.Ended, 0)
	sub.TrialStart = time.Unix(s.TrialStart, 0)
	sub.TrialEnd = time.Unix(s.TrialEnd, 0)

	sub.Quantity = int(s.Quantity)
	sub.Status = string(s.Status)
	sub.Metadata["user"] = s.Meta["user"]
	sub.Metadata["plan"] = s.Meta["plan"]

	return (*Sub)(s), nil
}

// Subscribe to a plan
func (c Client) CancelSubscription(sub *subscription.Subscription) error {
	err := c.Subs.Cancel(sub.StripeId, &stripe.SubParams{Customer: sub.StripeCustomerId})
	return err
}

// Do authorization, return token
func (c Client) Authorize(pay Payable) (*Token, error) {
	t, err := c.API.Tokens.New(&stripe.TokenParams{
		Card: ToCard(pay),
	})

	if err != nil {
		return nil, errors.New(err)
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
		return nil, errors.New(err)
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
		return nil, errors.New(err)
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
		return nil, errors.New(err)
	}

	return (*Customer)(customer), err
}

// Create new stripe customer
func (c Client) NewCustomer(user *user.User, token string) (*Customer, error) {
	params := &stripe.CustomerParams{
		Desc:  user.Name(),
		Email: user.Email,
	}

	if token != "" {
		params.SetSource(token)
	}

	// Update with our user metadata
	for k, v := range user.Metadata {
		params.AddMeta(k, json.Encode(v))
	}

	params.AddMeta("user", user.Id())

	customer, err := c.API.Customers.New(params)
	if err != nil {
		return nil, errors.New(err)
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
		return nil, errors.New(err)
	}

	return (*Card)(card), err
}

// Update card associated with Stripe customer
func (c Client) UpdateCard(token string, user *user.User) (*Card, error) {
	acct := user.Accounts.Stripe
	customerId := acct.CustomerId
	cardId := acct.CardId

	params := &stripe.CardParams{
		Customer: customerId,
		Token:    token,
	}

	card, err := c.API.Cards.Update(cardId, params)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Card)(card), err
}

func (c Client) GetCharge(chargeId string) (*Charge, error) {
	params := &stripe.ChargeParams{}
	params.Expand("balance_transaction")
	charge, err := c.API.Charges.Get(chargeId, params)
	if err != nil {
		return nil, err
	}

	return (*Charge)(charge), err
}

// Update Stripe charge
func (c Client) UpdateCharge(pay *payment.Payment) (*Charge, error) {
	pay.Metadata["order"] = pay.OrderId
	pay.Metadata["payment"] = pay.Id()
	pay.Metadata["user"] = pay.Buyer.UserId

	// Create params for update
	params := &stripe.ChargeParams{
		Desc: pay.Description,
		// Email: pay.Buyer.Email,
	}

	// Update metadata
	for k, v := range pay.Metadata {
		s, ok := v.(string)
		if ok {
			params.AddMeta(k, s)
		}
	}

	id := pay.Account.ChargeId

	charge, err := c.API.Charges.Update(id, params)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Charge)(charge), err
}

// Create new charge
func (c Client) NewCharge(source interface{}, pay *payment.Payment) (*Charge, error) {
	params := &stripe.ChargeParams{
		Amount:    uint64(pay.Amount),
		Currency:  stripe.Currency(pay.Currency),
		Customer:  pay.Account.CustomerId,
		Desc:      pay.Description,
		Fee:       uint64(pay.Fee),
		NoCapture: true,
		// Email:     pay.Buyer.Email,
	}

	// Update with our user metadata
	for k, v := range pay.Metadata {
		params.AddMeta(k, json.Encode(v))
	}

	params.AddMeta("order", pay.OrderId)
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

	params.Expand("balance_transaction")

	// Create charge
	ch, err := c.API.Charges.New(params)
	if err != nil {
		return nil, errors.New(err)
	}

	// Update charge Id on payment
	pay.Account.ChargeId = ch.ID

	return (*Charge)(ch), err
}

// Capture charge
func (c Client) Capture(id string) (*Charge, error) {
	log.Debug("Capture %v", id)
	ch, err := c.API.Charges.Capture(id, nil)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Charge)(ch), err
}
