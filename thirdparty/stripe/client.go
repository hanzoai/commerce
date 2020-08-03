package stripe

import (
	"context"
	"strconv"
	"time"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"

	"hanzo.io/log"
	"hanzo.io/models/deprecated/plan"
	"hanzo.io/models/deprecated/subscription"
	"hanzo.io/models/payment"
	"hanzo.io/models/transfer"
	"hanzo.io/models/types/accounts"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/refs"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/stripe/errors"
	"hanzo.io/util/json"
)

type Client struct {
	*client.API
	ctx context.Context
}

// Covert a payment model into a card card we can use for authorization
func PaymentToCard(pay *payment.Payment) *stripe.CardParams {
	card := stripe.CardParams{}
	card.Name = pay.Buyer.Name()
	card.Number = pay.Account.Number
	card.CVC = pay.Account.CVC
	card.Month = strconv.Itoa(pay.Account.Month)
	card.Year = strconv.Itoa(pay.Account.Year)
	card.Address1 = pay.Buyer.BillingAddress.Line1
	card.Address2 = pay.Buyer.BillingAddress.Line2
	card.City = pay.Buyer.BillingAddress.City
	card.State = pay.Buyer.BillingAddress.State
	card.Zip = pay.Buyer.BillingAddress.PostalCode
	card.Country = pay.Buyer.BillingAddress.Country
	return &card
}

func SubscriptionToCard(sub *subscription.Subscription) *stripe.CardParams {
	card := stripe.CardParams{}
	card.Name = sub.Buyer.Name()
	card.Number = sub.Account.Number
	card.CVC = sub.Account.CVC
	card.Month = strconv.Itoa(sub.Account.Month)
	card.Year = strconv.Itoa(sub.Account.Year)
	card.Address1 = sub.Buyer.BillingAddress.Line1
	card.Address2 = sub.Buyer.BillingAddress.Line2
	card.City = sub.Buyer.BillingAddress.City
	card.State = sub.Buyer.BillingAddress.State
	card.Zip = sub.Buyer.BillingAddress.PostalCode
	card.Country = sub.Buyer.BillingAddress.Country
	return &card
}

// Removed from API
// Create a Source object to pay with Bitcoin.
// func (c Client) CreateBitcoinSource(pay *payment.Payment, usr *user.User) (int64, string, string, error) {

// 	sourceParams := &stripe.SourceObjectParams{
// 		Type:     "bitcoin",
// 		Amount:   uint64(pay.Amount),
// 		Currency: "usd",
// 		Owner: &stripe.SourceOwnerParams{
// 			Email: usr.Email,
// 		},
// 	}

// 	src, err := c.API.Sources.New(sourceParams)

// 	log.JSON(src)

// 	if err != nil {
// 		return 0, "", "", err
// 	}

// 	return int64(src.TypeData["amount"].(float64)), src.TypeData["address"].(string), src.TypeData["uri"].(string), nil
// }

// func (c Client) ChargeBitcoinSource(pay *payment.Payment, src string) (bool, error) {
// 	chargeParams := &stripe.ChargeParams{
// 		Amount:   1000,
// 		Currency: "usd",
// 	}

// 	chargeParams.SetSource(src)
// 	ch, err := c.API.Charges.New(chargeParams)

// 	if err != nil {
// 		return false, err
// 	}

// 	return ch.Status == "succeeded", err
// }

func (c Client) NewSubscription(source interface{}, sub *subscription.Subscription) (*Sub, error) {
	log.Debug("sub.Plan %v", sub.Plan)
	params := &stripe.SubParams{
		Plan: sub.Plan.Id_,
	}

	switch v := source.(type) {
	case *Customer:
		params.Customer = v.ID
	case *user.User:
		params.Customer = v.Accounts.Stripe.CustomerId
		params.AddMeta("user", v.Id())
	}

	params.AddMeta("plan", sub.Plan.Id_)

	s, err := c.Subs.New(params)
	if err != nil {
		return nil, errors.New(err)
	}

	sub.Ref.Stripe.Id = s.ID
	sub.Ref.Type = refs.StripeEcommerceRefType
	sub.Account.CustomerId = s.Customer.ID
	sub.Account.Type = accounts.StripeType
	sub.FeePercent = s.FeePercent
	sub.EndCancel = s.EndCancel
	sub.PeriodStart = time.Unix(s.PeriodStart, 0)
	sub.PeriodEnd = time.Unix(s.PeriodEnd, 0)
	// sub.Start = time.Unix(s.Start, 0)
	sub.Ended = time.Unix(s.Ended, 0)
	sub.TrialStart = time.Unix(s.TrialStart, 0)
	sub.TrialEnd = time.Unix(s.TrialEnd, 0)

	sub.Quantity = int(s.Quantity)
	sub.Status = subscription.Status(s.Status)

	return (*Sub)(s), nil
}

// Update subscribe to a plan
func (c Client) UpdateSubscription(sub *subscription.Subscription) (*Sub, error) {
	params := &stripe.SubParams{
		Customer: sub.Account.CustomerId,
		Plan:     sub.Plan.Id_,
		Quantity: uint64(sub.Quantity),
	}

	params.AddMeta("plan", sub.Plan.Id_)

	s, err := c.Subs.Update(sub.Ref.Stripe.Id, params)
	if err != nil {
		return nil, errors.New(err)
	}

	sub.Ref.Stripe.Id = s.ID
	sub.Ref.Type = refs.StripeEcommerceRefType
	sub.Account.CustomerId = s.Customer.ID
	sub.Account.Type = accounts.StripeType
	sub.FeePercent = s.FeePercent
	sub.EndCancel = s.EndCancel
	sub.PeriodStart = time.Unix(s.PeriodStart, 0)
	sub.PeriodEnd = time.Unix(s.PeriodEnd, 0)
	// sub.Start = time.Unix(s.Start, 0)
	sub.Ended = sub.PeriodEnd
	sub.TrialStart = time.Unix(s.TrialStart, 0)
	sub.TrialEnd = time.Unix(s.TrialEnd, 0)

	sub.Quantity = int(s.Quantity)
	sub.Status = subscription.Status(s.Status)

	return (*Sub)(s), nil
}

// Subscribe to a plan
func (c Client) CancelSubscription(sub *subscription.Subscription) (*Sub, error) {
	params := &stripe.SubParams{
		Customer:  sub.Account.CustomerId,
		EndCancel: true,
	}

	s, err := c.Subs.Cancel(sub.Ref.Stripe.Id, params)
	if err != nil {
		return nil, errors.New(err)
	}

	sub.Ref.Stripe.Id = s.ID
	sub.Ref.Type = refs.StripeEcommerceRefType
	sub.Account.CustomerId = s.Customer.ID
	sub.Account.Type = accounts.StripeType
	sub.FeePercent = s.FeePercent
	sub.EndCancel = s.EndCancel
	sub.PeriodStart = time.Unix(s.PeriodStart, 0)
	sub.PeriodEnd = time.Unix(s.PeriodEnd, 0)
	// sub.Start = time.Unix(s.Start, 0)
	sub.Ended = sub.PeriodEnd
	sub.TrialStart = time.Unix(s.TrialStart, 0)
	sub.TrialEnd = time.Unix(s.TrialEnd, 0)
	sub.CanceledAt = time.Now()
	sub.EndCancel = true

	sub.Quantity = int(s.Quantity)
	sub.Status = subscription.Status(s.Status)

	return (*Sub)(s), nil
}

// Do authorization, return token
func (c Client) Authorize(pay *payment.Payment) (*Token, error) {
	crd := PaymentToCard(pay)
	log.JSON(crd)

	t, err := c.API.Tokens.New(&stripe.TokenParams{
		Card: crd,
	})

	if err != nil {
		return nil, errors.New(err)
	}

	// Cast back to our token
	return (*Token)(t), nil
}

func (c Client) AuthorizeSubscription(sub *subscription.Subscription) (*Token, error) {

	t, err := c.API.Tokens.New(&stripe.TokenParams{
		Card: SubscriptionToCard(sub),
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
	// for k, v := range usr.Metadata {
	// 	params.AddMeta(k, json.Encode(v))
	// }

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
func (c Client) NewCard(token string, usr *user.User) (*Card, error) {
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

// Add new subscription to Stripe
func (c Client) NewPlan(p *plan.Plan) (*Plan, error) {
	params := &stripe.PlanParams{
		ID:            p.Id(),
		Name:          p.Name,
		Currency:      stripe.Currency(p.Currency),
		Interval:      stripe.PlanInterval(p.Interval),
		IntervalCount: uint64(p.IntervalCount),
		TrialPeriod:   uint64(p.TrialPeriodDays),
		Statement:     p.Description,
	}

	params.AddMeta("plan", p.Id())

	plan, err := c.API.Plans.New(params)
	if err != nil {
		return nil, errors.New(err)
	}

	p.Ref.Stripe.Id = plan.ID
	p.Ref.Type = refs.StripeEcommerceRefType

	return (*Plan)(plan), nil
}

func (c Client) UpdatePlan(p *plan.Plan) (*Plan, error) {
	planId := p.Id()

	params := &stripe.PlanParams{
		ID:            p.Id(),
		Name:          p.Name,
		Currency:      stripe.Currency(p.Currency),
		Interval:      stripe.PlanInterval(p.Interval),
		IntervalCount: uint64(p.IntervalCount),
		TrialPeriod:   uint64(p.TrialPeriodDays),
		Statement:     p.Description,
	}

	plan, err := c.API.Plans.Update(planId, params)
	if err != nil {
		return nil, errors.New(err)
	}

	p.Ref.Stripe.Id = plan.ID
	p.Ref.Type = refs.StripeEcommerceRefType

	return (*Plan)(plan), nil
}

// func (c Client) DeletePlan(p *plan.Plan) (*Plan, error) {
// 	params := &stripe.PlanParams {
// 		ID: p.Id(),
// 		Name: p.Name,
// 		Currency: stripe.Currency(p.Currency),
// 		Interval: stripe.PlanInterval(p.Interval),
// 		IntervalCount: uint64(p.IntervalCount),
// 		TrialPeriod: uint64(p.TrialPeriodDays),
// 		Statement: p.Description,
// 	}
// 	plan, err := c.API.Plans.Del(p.Id(), params)
// 	if err != nil {
// 		return nil, errors.New(err)
// 	}

// 	return (*Plan)(plan), nil
// }

// Update card associated with Stripe customer
func (c Client) UpdateCard(token string, usr *user.User) (*Card, error) {
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
		log.Error("Payment had nil metadata somehow: %#v", pay, c.ctx)
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
		Meta:     map[string]string{"description": tr.Description},
	}
	params.Params.IdempotencyKey = tid

	if tr.AffiliateId != "" {
		params.AddMeta("affiliate", tr.AffiliateId)
	}
	if tr.PartnerId != "" {
		params.AddMeta("affiliate", tr.AffiliateId)
	}
	params.AddMeta("transfer", tid)
	params.AddMeta("fee", tr.FeeId)

	// Create transfer
	str, err := c.API.Transfers.New(params)
	if err != nil {
		return nil, errors.New(err)
	}

	t := (*Transfer)(str)
	UpdateTransferFromStripe(tr, t)

	return t, err
}

func (c Client) Payout(tr *transfer.Transfer) (*Payout, error) {
	tid := tr.Id()
	params := &stripe.PayoutParams{
		Amount:              int64(tr.Amount),
		Destination:         tr.Destination,
		Currency:            stripe.Currency(tr.Currency),
		StatementDescriptor: tr.Description,
	}
	params.Params.IdempotencyKey = tid

	if tr.AffiliateId != "" {
		params.AddMeta("affiliate", tr.AffiliateId)
	}
	if tr.PartnerId != "" {
		params.AddMeta("affiliate", tr.AffiliateId)
	}
	params.AddMeta("payout", tid)
	params.AddMeta("fee", tr.FeeId)

	// Create transfer
	str, err := c.API.Payouts.New(params)
	if err != nil {
		return nil, errors.New(err)
	}

	t := (*Payout)(str)
	UpdatePayoutFromStripe(tr, t)

	return t, err
}
