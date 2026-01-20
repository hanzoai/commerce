package stripe

import (
	"context"
	"strconv"
	"time"

	"github.com/stripe/stripe-go/v75"
	"github.com/stripe/stripe-go/v75/client"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transfer"
	"github.com/hanzoai/commerce/models/types/accounts"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/types/refs"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/thirdparty/stripe/errors"
	"github.com/hanzoai/commerce/util/json"
)

type Client struct {
	*client.API
	ctx context.Context
}

func toPtr[T any](v T) *T {
	return &v
}

// Covert a payment model into a card card we can use for authorization
func PaymentToCard(pay *payment.Payment) *stripe.CardParams {
	card := &stripe.CardParams{
		Name:           toPtr(pay.Buyer.Name()),
		Number:         toPtr(pay.Account.Number),
		CVC:            toPtr(pay.Account.CVC),
		ExpMonth:       toPtr(strconv.Itoa(pay.Account.Month)),
		ExpYear:        toPtr(strconv.Itoa(pay.Account.Year)),
		AddressLine1:   toPtr(pay.Buyer.BillingAddress.Line1),
		AddressLine2:   toPtr(pay.Buyer.BillingAddress.Line2),
		AddressCity:    toPtr(pay.Buyer.BillingAddress.City),
		AddressState:   toPtr(pay.Buyer.BillingAddress.State),
		AddressZip:     toPtr(pay.Buyer.BillingAddress.PostalCode),
		AddressCountry: toPtr(pay.Buyer.BillingAddress.Country),
	}
	return card
}

func SubscriptionToCard(sub *subscription.Subscription) *stripe.CardParams {
	card := &stripe.CardParams{
		Name:           toPtr(sub.Buyer.Name()),
		Number:         toPtr(sub.Account.Number),
		CVC:            toPtr(sub.Account.CVC),
		ExpMonth:       toPtr(strconv.Itoa(sub.Account.Month)),
		ExpYear:        toPtr(strconv.Itoa(sub.Account.Year)),
		AddressLine1:   toPtr(sub.Buyer.BillingAddress.Line1),
		AddressLine2:   toPtr(sub.Buyer.BillingAddress.Line2),
		AddressCity:    toPtr(sub.Buyer.BillingAddress.City),
		AddressState:   toPtr(sub.Buyer.BillingAddress.State),
		AddressZip:     toPtr(sub.Buyer.BillingAddress.PostalCode),
		AddressCountry: toPtr(sub.Buyer.BillingAddress.Country),
	}
	return card
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

func (c Client) NewSubscription(source interface{}, sub *subscription.Subscription) (*Subscription, error) {
	log.Debug("sub.Plan %v", sub.Plan)
	params := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{
			{
				Plan: stripe.String(sub.Plan.Id_),
			},
		},
	}

	switch v := source.(type) {
	case *Customer:
		params.Customer = stripe.String(v.ID)
	case *user.User:
		params.Customer = stripe.String(v.Accounts.Stripe.CustomerId)
		params.AddMetadata("user", v.Id())
	}

	params.AddMetadata("plan", sub.Plan.Id_)

	s, err := c.API.Subscriptions.New(params)
	if err != nil {
		return nil, errors.New(err)
	}

	sub.Ref.Stripe.Id = s.ID
	sub.Ref.Type = refs.StripeEcommerceRefType
	sub.Account.CustomerId = s.Customer.ID
	sub.Account.Type = accounts.StripeType
	sub.FeePercent = s.ApplicationFeePercent
	sub.EndCancel = s.CancelAtPeriodEnd
	sub.PeriodStart = time.Unix(s.CurrentPeriodStart, 0)
	sub.PeriodEnd = time.Unix(s.CurrentPeriodEnd, 0)
	// sub.Start = time.Unix(s.Start, 0)
	sub.Ended = time.Unix(s.EndedAt, 0)
	sub.TrialStart = time.Unix(s.TrialStart, 0)
	sub.TrialEnd = time.Unix(s.TrialEnd, 0)

	if len(s.Items.Data) > 0 {
		sub.Quantity = int(s.Items.Data[0].Quantity)
	}
	sub.Status = subscription.Status(s.Status)

	return (*Subscription)(s), nil
} // Update subscribe to a plan
func (c Client) UpdateSubscription(sub *subscription.Subscription) (*Subscription, error) {
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(sub.Account.CustomerId),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Plan:     stripe.String(sub.Plan.Id_),
				Quantity: stripe.Int64(int64(sub.Quantity)),
			},
		},
	}

	params.AddMetadata("plan", sub.Plan.Id_)

	s, err := c.API.Subscriptions.Update(sub.Ref.Stripe.Id, params)
	if err != nil {
		return nil, errors.New(err)
	}

	sub.Ref.Stripe.Id = s.ID
	sub.Ref.Type = refs.StripeEcommerceRefType
	sub.Account.CustomerId = s.Customer.ID
	sub.Account.Type = accounts.StripeType
	sub.FeePercent = s.ApplicationFeePercent
	sub.EndCancel = s.CancelAtPeriodEnd
	sub.PeriodStart = time.Unix(s.CurrentPeriodStart, 0)
	sub.PeriodEnd = time.Unix(s.CurrentPeriodEnd, 0)
	// sub.Start = time.Unix(s.Start, 0)
	sub.Ended = time.Unix(s.EndedAt, 0)
	sub.TrialStart = time.Unix(s.TrialStart, 0)
	sub.TrialEnd = time.Unix(s.TrialEnd, 0)

	if len(s.Items.Data) > 0 {
		sub.Quantity = int(s.Items.Data[0].Quantity)
	}
	sub.Status = subscription.Status(s.Status)

	return (*Subscription)(s), nil
}

// Subscribe to a plan
func (c Client) CancelSubscription(sub *subscription.Subscription) (*Subscription, error) {
	params := &stripe.SubscriptionCancelParams{}

	s, err := c.API.Subscriptions.Cancel(sub.Ref.Stripe.Id, params)
	if err != nil {
		return nil, errors.New(err)
	}

	sub.Ref.Stripe.Id = s.ID
	sub.Ref.Type = refs.StripeEcommerceRefType
	sub.Account.CustomerId = s.Customer.ID
	sub.Account.Type = accounts.StripeType
	sub.FeePercent = s.ApplicationFeePercent
	sub.EndCancel = s.CancelAtPeriodEnd
	sub.PeriodStart = time.Unix(s.CurrentPeriodStart, 0)
	sub.PeriodEnd = time.Unix(s.CurrentPeriodEnd, 0)
	// sub.Start = time.Unix(s.Start, 0)
	sub.Ended = time.Unix(s.EndedAt, 0)
	sub.TrialStart = time.Unix(s.TrialStart, 0)
	sub.TrialEnd = time.Unix(s.TrialEnd, 0)
	sub.CanceledAt = time.Unix(s.CanceledAt, 0)

	if len(s.Items.Data) > 0 {
		sub.Quantity = int(s.Items.Data[0].Quantity)
	}
	sub.Status = subscription.Status(s.Status)

	return (*Subscription)(s), nil
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
		Charge: stripe.String(pay.Account.ChargeId),
		Amount: stripe.Int64(int64(refundAmount)),
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
		Customer: stripe.String(customerId),
	}

	card, err := c.API.Cards.Get(cardId, params)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Card)(card), nil
}

// Get Stripe customer
func (c Client) GetCustomer(token string, usr *user.User) (*Customer, error) {
	params := &stripe.CustomerParams{}
	params.SetStripeAccount(token)

	cust, err := c.API.Customers.Get(usr.Accounts.Stripe.CustomerId, params)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Customer)(cust), nil
}

// Update Stripe customer
func (c Client) UpdateCustomer(usr *user.User) (*Customer, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(usr.Email),
	}

	// Update Default source
	if usr.Accounts.Stripe.CardId != "" {
		params.DefaultSource = stripe.String(usr.Accounts.Stripe.CardId)
	}

	// Update with our user metadata
	// for k, v := range usr.Metadata {
	// 	params.AddMetadata(k, json.Encode(v))
	// }

	params.AddMetadata("user", usr.Id())

	customerId := usr.Accounts.Stripe.CustomerId

	cust, err := c.API.Customers.Update(customerId, params)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Customer)(cust), nil
} // Create new stripe customer
func (c Client) NewCustomer(token string, user *user.User) (*Customer, error) {
	params := &stripe.CustomerParams{
		Description: stripe.String(user.Name()),
		Email:       stripe.String(user.Email),
	}
	params.Source = &token

	// Update with our user metadata
	// for k, v := range user.Metadata {
	// 	params.AddMeta(k, json.Encode(v))
	// }

	params.AddMetadata("user", user.Id())

	cust, err := c.API.Customers.New(params)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Customer)(cust), nil
} // Add new card to Stripe customer
func (c Client) NewCard(token string, usr *user.User) (*Card, error) {
	params := &stripe.CardParams{
		Customer: stripe.String(usr.Accounts.Stripe.CustomerId),
		Token:    stripe.String(token),
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
		ID:              stripe.String(p.Id()),
		Nickname:        stripe.String(p.Name),
		Currency:        stripe.String(string(p.Currency)),
		Interval:        stripe.String(string(p.Interval)),
		IntervalCount:   stripe.Int64(int64(p.IntervalCount)),
		TrialPeriodDays: stripe.Int64(int64(p.TrialPeriodDays)),
		Product: &stripe.PlanProductParams{
			Name: stripe.String(p.Name),
		},
	}

	params.AddMetadata("plan", p.Id())

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
		ID:              stripe.String(p.Id()),
		Nickname:        stripe.String(p.Name),
		Currency:        stripe.String(string(p.Currency)),
		Interval:        stripe.String(string(p.Interval)),
		IntervalCount:   stripe.Int64(int64(p.IntervalCount)),
		TrialPeriodDays: stripe.Int64(int64(p.TrialPeriodDays)),
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
		Customer: stripe.String(customerId),
		Token:    stripe.String(token),
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
		Customer: stripe.String(usr.Accounts.Stripe.CustomerId),
	}

	card, err := c.API.Cards.Del(cardId, params)
	if err != nil {
		return nil, errors.New(err)
	}

	return (*Card)(card), nil
}

func (c Client) GetCharge(chargeId string) (*Charge, error) {
	params := &stripe.ChargeParams{
		Expand: []*string{stripe.String("balance_transaction")},
	}

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
		Description: stripe.String(pay.Description),
		// Email: pay.Buyer.Email,
	}

	// Update metadata
	for k, v := range pay.Metadata {
		s, ok := v.(string)
		if ok {
			params.AddMetadata(k, s)
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
		Amount:      stripe.Int64(int64(pay.Amount)),
		Currency:    stripe.String(string(pay.Currency)),
		Customer:    stripe.String(pay.Account.CustomerId),
		Description: stripe.String(pay.Description),
		Capture:     stripe.Bool(false),
		// Email:     pay.Buyer.Email,
	}

	// Update with our user metadata
	for k, v := range pay.Metadata {
		params.AddMetadata(k, json.Encode(v))
	}

	params.AddMetadata("order", pay.OrderId)
	params.AddMetadata("payment", pay.Id())

	switch v := source.(type) {
	case string:
		params.SetSource(v)
	case *Customer:
		params.Customer = stripe.String(v.ID)
	case *user.User:
		params.Customer = stripe.String(v.Accounts.Stripe.CustomerId)
		params.AddMetadata("user", v.Id())
	}

	params.AddExpand("balance_transaction")

	// Create charge
	ch, err := c.API.Charges.New(params)
	if err != nil {
		return nil, err
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
		Amount:      stripe.Int64(int64(tr.Amount)),
		Destination: stripe.String(tr.Destination),
		Currency:    stripe.String(string(tr.Currency)),
		Description: stripe.String(tr.Description),
	}
	params.SetIdempotencyKey(tid)

	if tr.AffiliateId != "" {
		params.AddMetadata("affiliate", tr.AffiliateId)
	}
	if tr.PartnerId != "" {
		params.AddMetadata("partner", tr.PartnerId)
	}
	params.AddMetadata("transfer", tid)
	params.AddMetadata("fee", tr.FeeId)

	// Create transfer
	str, err := c.API.Transfers.New(params)
	if err != nil {
		return nil, err
	}

	t := (*Transfer)(str)
	UpdateTransferFromStripe(tr, t)

	return t, nil
}

func (c Client) Payout(tr *transfer.Transfer) (*Payout, error) {
	tid := tr.Id()
	params := &stripe.PayoutParams{
		Amount:              stripe.Int64(int64(tr.Amount)),
		Destination:         stripe.String(tr.Destination),
		Currency:            stripe.String(string(tr.Currency)),
		StatementDescriptor: stripe.String(tr.Description),
	}
	params.SetIdempotencyKey(tid)

	if tr.AffiliateId != "" {
		params.AddMetadata("affiliate", tr.AffiliateId)
	}
	if tr.PartnerId != "" {
		params.AddMetadata("partner", tr.PartnerId)
	}
	params.AddMetadata("payout", tid)
	params.AddMetadata("fee", tr.FeeId)

	// Create payout
	str, err := c.API.Payouts.New(params)
	if err != nil {
		return nil, err
	}

	t := (*Payout)(str)
	UpdatePayoutFromStripe(tr, t)

	return t, nil
}
