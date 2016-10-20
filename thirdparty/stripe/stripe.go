package stripe

import (
	"strconv"
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"

	"crowdstart.com/models/payment"
	"crowdstart.com/models/transfer"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/stripe/errors"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
)

type Client struct {
	*client.API
	ctx appengine.Context
}

func New(ctx appengine.Context, accessToken string) *Client {
	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(ctx)
	httpClient.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(55) * time.Second,
	}
	stripe.SetBackend(stripe.APIBackend, nil)
	stripe.SetHTTPClient(httpClient)

	sc := &client.API{}
	sc.Init(accessToken, nil)
	return &Client{sc, ctx}
}

// Enable debug logging in development
func init() {
	if appengine.IsDevAppServer() {
		stripe.LogLevel = 2
	}
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
		return nil, errors.New(err)
	}

	// Cast back to our token
	return (*Token)(t), nil
}

// Attempts to refund payment and updates the payment in datastore
func (c Client) RefundPayment(pay *payment.Payment, refundAmount currency.Cents) (*payment.Payment, error) {
	if refundAmount > pay.Amount {
		return pay, errors.RefundGreaterThanPayment
	}

	if refundAmount+pay.AmountRefunded > pay.Amount {
		return pay, errors.RefundGreaterThanPayment
	}

	if pay.Status == payment.Unpaid {
		return pay, errors.UnableToRefundUnpaidTransaction
	}

	// Process refund with Stripe
	refund, err := c.API.Refunds.New(&stripe.RefundParams{
		Charge: pay.Account.ChargeId,
		Amount: uint64(refundAmount),
	})

	if err != nil {
		log.Error("Error refunding payment %s", err.Error())
		return pay, err
	}

	// Update payment
	pay.AmountRefunded = currency.Cents(refund.Amount)
	if pay.AmountRefunded == pay.Amount {
		pay.Status = payment.Refunded
	}

	return pay, pay.Put()
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

	return (*Card)(card), nil
}

// Get Stripe customer
func (c Client) GetCustomer(token, usr *user.User) (*Customer, error) {
	params := &stripe.CustomerParams{}
	params.SetSource(token)

	cust, err := c.API.Customers.Get(usr.Accounts.Stripe.CustomerId, params)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Customer)(cust), nil
}

// Update Stripe customer
func (c Client) UpdateCustomer(usr *user.User) (*Customer, error) {
	params := &stripe.CustomerParams{
		Email: usr.Email,
	}

	// Update Default source
	if usr.Accounts.Stripe.CardId != "" {
		params.DefaultSource = usr.Accounts.Stripe.CardId
	}

	// Update with our user metadata
	for k, v := range usr.Metadata {
		params.AddMeta(k, json.Encode(v))
	}

	params.AddMeta("user", usr.Id())

	customerId := usr.Accounts.Stripe.CustomerId

	cust, err := c.API.Customers.Update(customerId, params)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Customer)(cust), nil
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

	cust, err := c.API.Customers.New(params)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Customer)(cust), nil
}

// Add new card to Stripe customer
func (c Client) AddCard(token string, usr *user.User) (*Card, error) {
	params := &stripe.CardParams{
		Customer: usr.Accounts.Stripe.CustomerId,
		Token:    token,
	}

	card, err := c.API.Cards.New(params)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Card)(card), nil
}

// Update card associated with Stripe customer
func (c Client) UpdateCard(token string, pay *payment.Payment, usr *user.User) (*Card, error) {
	customerId := usr.Accounts.Stripe.CustomerId
	cardId := usr.Accounts.Stripe.CardId

	params := &stripe.CardParams{
		Customer: customerId,
		Token:    token,
	}

	card, err := c.API.Cards.Update(cardId, params)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Card)(card), nil
}

// Update card associated with Stripe customer
func (c Client) DeleteCard(cardId string, usr *user.User) (*Card, error) {
	params := &stripe.CardParams{
		Customer: usr.Accounts.Stripe.CustomerId,
	}

	card, err := c.API.Cards.Del(cardId, params)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Card)(card), nil
}

func (c Client) GetCharge(chargeId string) (*Charge, error) {
	params := &stripe.ChargeParams{}
	params.Expand("balance_transaction")

	charge, err := c.API.Charges.Get(chargeId, params)
	if err != nil {
		return nil, err
	}

	return (*Charge)(charge), nil
}

// Update Stripe charge
func (c Client) UpdateCharge(pay *payment.Payment) (*Charge, error) {
	// TODO: How is this ever nil?
	if pay.Metadata == nil {
		log.Error("Payment had nil metadata somehow: %v", pay, c.ctx)
		pay.Metadata = make(map[string]interface{})
	}
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

	return (*Charge)(charge), nil
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

	return (*Charge)(ch), nil
}

// Capture charge
func (c Client) Capture(chargeId string) (*Charge, error) {
	log.Debug("Capture charge '%s'", chargeId)

	ch, err := c.API.Charges.Capture(chargeId, nil)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Charge)(ch), nil
}

func (c Client) Transfer(tr *transfer.Transfer) (*Transfer, error) {
	tid := tr.Id()
	params := &stripe.TransferParams{
		Amount:   int64(tr.Amount),
		Dest:     tr.Destination,
		Currency: stripe.Currency(tr.Currency),
		Desc:     tr.Description,
	}
	params.Params.IdempotencyKey = tid

	params.AddMeta("affiliate", tr.AffiliateId)
	params.AddMeta("transfer", tid)

	// Create transfer
	str, err := c.API.Transfers.New(params)
	if err != nil {
		return nil, errors.New(err)
	}

	t := (*Transfer)(str)
	UpdateTransferFromStripe(tr, t)

	return t, err
}
