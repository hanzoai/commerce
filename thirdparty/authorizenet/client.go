package stripe

import (
	"strconv"

	"github.com/hunterlong/authorizecim"

	"hanzo.io/log"
	"hanzo.io/models/payment"
	"hanzo.io/models/subscription"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/refs"
	"hanzo.io/models/plan"
	"hanzo.io/thirdparty/authorizenet/errors"
)

type Client struct {

}

func toStringExpirationDate(month int, year int) string {

	if(month < 10) {
		return "0" + strconv.Itoa(month) + "/" + strconv.Itoa(year)
	}
	return strconv.Itoa(month) + "/" + strconv.Itoa(year)

}

func HanzoToAuthorizeSubscription(sub *subscription.Subscription) *AuthorizeCIM.Subscription {

	interval := AuthorizeCIM.IntervalMonthly()
	if sub.Plan.Interval == plan.Yearly {
		interval = AuthorizeCIM.IntervalYearly()
	}
	subscription := AuthorizeCIM.Subscription{
		Name:		 sub.Plan.Name,
		Amount:      sub.Plan.Currency.ToStringNoSymbol(sub.Plan.Price),
		TrialAmount: "0.00",
		PaymentSchedule: &AuthorizeCIM.PaymentSchedule{
			StartDate:        sub.PeriodStart.String(),
			TotalOccurrences: strconv.Itoa(sub.PeriodsRemaining()),
			TrialOccurrences: strconv.Itoa(sub.TrialPeriodsRemaining()),
			Interval: interval,
		},
		Payment: &AuthorizeCIM.Payment{
			CreditCard: AuthorizeCIM.CreditCard{
				CardNumber:     sub.Account.Number,
				ExpirationDate: toStringExpirationDate(sub.Account.Month, sub.Account.Year),
				CardCode:		sub.Account.CVC,
			},
		},
		BillTo: &AuthorizeCIM.BillTo{
			Address:     sub.Buyer.Address.Line1,
			City:        sub.Buyer.Address.City,
			State:       sub.Buyer.Address.State,
			Zip:         sub.Buyer.Address.PostalCode,
			Country:     sub.Buyer.Address.Country,
		},
	}
	return &subscription
}
// Covert a payment model into a card card we can use for authorization
func PaymentToNewTransaction(pay *payment.Payment) *AuthorizeCIM.NewTransaction{
	newTransaction := AuthorizeCIM.NewTransaction{
				Amount: pay.Currency.ToStringNoSymbol(pay.Amount),
				RefTransId: pay.Account.RefTransId,
				CreditCard: AuthorizeCIM.CreditCard{
					CardNumber:     pay.Account.Number,
					ExpirationDate: toStringExpirationDate(pay.Account.Month, pay.Account.Year),
					CardCode:		pay.Account.CVC,
				},
				BillTo: &AuthorizeCIM.BillTo{
					Address:     pay.Buyer.Address.Line1,
					City:        pay.Buyer.Address.City,
					State:       pay.Buyer.Address.State,
					Zip:         pay.Buyer.Address.PostalCode,
					Country:     pay.Buyer.Address.Country,
				},
			}
	return &newTransaction
}

func PaymentToPreviousTransaction(pay *payment.Payment) *AuthorizeCIM.PreviousTransaction{
	prevTransaction := AuthorizeCIM.PreviousTransaction{
				Amount: pay.Currency.ToStringNoSymbol(pay.Amount),
				RefId: pay.Account.RefTransId,
			}
	return &prevTransaction
}

func PopulatePaymentWithResponse(pay *payment.Payment, tran *AuthorizeCIM.TransactionResponse) *payment.Payment {
	pay.Account.AuthCode = tran.Response.AuthCode
	pay.Account.AvsResultCode = tran.Response.AvsResultCode
	pay.Account.CvvResultCode = tran.Response.CvvResultCode
	pay.Account.CavvResultCode = tran.Response.CavvResultCode
	pay.Account.TransId = tran.Response.TransID
	pay.Account.RefTransId = tran.Response.RefTransID
	pay.Account.TransHash = tran.Response.TransHash
	pay.Account.TestRequest = tran.Response.TestRequest
	pay.Account.AccountNumber = tran.Response.AccountNumber
	pay.Account.AccountType = tran.Response.AccountType

	return pay
}

func PopulateSubscriptionWithResponse(sub *subscription.Subscription, tran *AuthorizeCIM.SubscriptionResponse) *subscription.Subscription {
	sub.Ref.Affirm.Id = tran.SubscriptionID
	sub.Ref.Type = refs.AffirmEcommerceRefType

	return sub
}

/*func SubscriptionToCard(sub *subscription.Subscription) *stripe.CardParams {
	card := stripe.CardParams{}
	card.Name = sub.Buyer.Name()
	card.Number = sub.StripeAccount.Number
	card.CVC = sub.StripeAccount.CVC
	card.Month = strconv.Itoa(sub.StripeAccount.Month)
	card.Year = strconv.Itoa(sub.StripeAccount.Year)
	card.Address1 = sub.Buyer.Address.Line1
	card.Address2 = sub.Buyer.Address.Line2
	card.City = sub.Buyer.Address.City
	card.State = sub.Buyer.Address.State
	card.Zip = sub.Buyer.Address.PostalCode
	card.Country = sub.Buyer.Address.Country
	return &card
}*/


func (c Client) NewSubscription(token string, sub *subscription.Subscription) (*subscription.Subscription, error) {
	log.Debug("sub.Plan %v", sub.Plan)

	subscription := HanzoToAuthorizeSubscription(sub)

	response, err := subscription.Charge()

	if response.Approved() {
		return PopulateSubscriptionWithResponse(sub, response), nil
	}
	return sub, err
}

// Update subscribe to a plan
func (c Client) UpdateSubscription(sub *subscription.Subscription) (*subscription.Subscription, error) {
	log.Debug("sub.Plan %v", sub.Plan)

	subscription := HanzoToAuthorizeSubscription(sub)

	response, err := subscription.Update()

	if response.Approved() {
		return PopulateSubscriptionWithResponse(sub, response), nil
	}
	return sub, err
}

// Subscribe to a plan
func (c Client) CancelSubscription(sub *subscription.Subscription) (*subscription.Subscription, error) {
	log.Debug("sub.Plan %v", sub.Plan)

	s := AuthorizeCIM.SetSubscription{
		Id: sub.Ref.Affirm.Id,
	}
	_, err := s.Cancel()

	if err == nil {
		sub.Canceled = true
		sub.Status = subscription.Canceled
		return sub, nil
	}
	return sub, err
}

// Do authorization, return token
func (c Client) Authorize(pay *payment.Payment) (*payment.Payment, error) {
	newTransaction := PaymentToNewTransaction(pay)

	response, err := newTransaction.AuthOnly()

	if response.Approved() {
		pay = PopulatePaymentWithResponse(pay,response)
		return pay, nil
	} else {
		return pay, err
	}
}

/*func (c Client) AuthorizeSubscription(sub *subscription.Subscription) (*AuthorizeCIM.TransactionResponse, error) {

	t, err := c.API.Tokens.New(&stripe.TokenParams{
		Card: SubscriptionToCard(sub),
	})

	if err != nil {
		return nil, errors.New(err)
	}

	// Cast back to our token
	return (*Token)(t), nil
}*/

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

	newTransaction := PaymentToNewTransaction(pay)
	newTransaction.Amount = pay.Currency.ToStringNoSymbol(refundAmount)

	response, err := newTransaction.Refund()
	if response.Approved() {
		// Authorize.Net does not return the specific amount
		// refunded in this transaction. If the response is
		// approved you can only assume things went fine.
		pay.AmountRefunded = currency.Cents(refundAmount)
		if pay.AmountRefunded == pay.Amount {
			pay.Status = payment.Refunded
		}
		return pay, pay.Put()
	} else {
		return pay, err
	}
}

/*func (c Client) GetCard(cardId string, customerId string) (*Card, error) {
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
}*/

// Create new charge
func (c Client) NewCharge(source interface{}, pay *payment.Payment) (*payment.Payment, error) {

	newTransaction := PaymentToNewTransaction(pay)

	response, err := newTransaction.AuthOnly()

	if response.Approved() {
		pay = PopulatePaymentWithResponse(pay,response)
		return pay, nil
	} else {
		return pay, err
	}
}

// Capture charge
func (c Client) Capture(pay *payment.Payment) (*payment.Payment, error) {
	log.Debug("Capture charge '%s'", pay.Account.AuthCode)
	oldTransaction := PaymentToPreviousTransaction(pay)

	response, err := oldTransaction.Capture()
	if response.Approved() {
		pay = PopulatePaymentWithResponse(pay,response)
		return pay, nil
	} else {
		return pay, err
	}

}
