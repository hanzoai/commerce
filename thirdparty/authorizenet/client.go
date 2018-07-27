package authorizenet

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"encoding/json"
	"net/http"
	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
	"net/http/httputil"
	"bytes"
	"io/ioutil"

	"github.com/hanzoai/goauthorizenet"

	"hanzo.io/log"
	"hanzo.io/models"
	"hanzo.io/models/order"
	"hanzo.io/models/payment"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/refs"
	json2 "hanzo.io/util/json"
)

type Client struct {
	client *http.Client
	ctx context.Context
	loginId string
	transactionKey string
	Key string
	test bool
}

func New(ctx context.Context, loginId string, transactionKey string, key string, test bool) *Client {

	// Set deadline
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(ctx)

	httpClient.Transport = &urlfetch.Transport{
		Context: ctx,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}

	return &Client{httpClient, ctx, loginId, transactionKey, key, test}

}

func (c Client) getTestValue() string {
	if(c.test) {
		return "test"
	}
	return "live"
}

func ToStringExpirationDate(month int, year int) string {

	y := strconv.Itoa(year)
	l := len(y)
	twoDigitYear := y
	if  l > 2 {
		twoDigitYear = y[l-2:]
	}

	if(month < 10) {
		return "0" + strconv.Itoa(month) + "/" + twoDigitYear
	}
	return strconv.Itoa(month) + "/" + twoDigitYear
}

func HanzoToAuthorizeSubscription(sub *order.Subscription) *AuthorizeCIM.Subscription {

	interval := AuthorizeCIM.IntervalMonthly()
	if sub.Interval == models.Yearly {
		interval = AuthorizeCIM.IntervalYearly()
	}
	subscription := AuthorizeCIM.Subscription{
		Name:		 sub.ProductId + sub.PlanId,
		Amount:      sub.Currency.ToStringNoSymbol(sub.Price),
		TrialAmount: "0.00",
		PaymentSchedule: &AuthorizeCIM.PaymentSchedule{
			StartDate:        sub.PeriodStart.Format("2006-01-02"),
			TotalOccurrences: "9999",
			TrialOccurrences: strconv.Itoa(sub.TrialPeriodsRemaining()),
			Interval:		  interval,
		},
		Payment: &AuthorizeCIM.Payment{
			CreditCard: AuthorizeCIM.CreditCard{
				CardNumber:     sub.Account.Number,
				ExpirationDate: ToStringExpirationDate(sub.Account.Month, sub.Account.Year),
				CardCode:		sub.Account.CVC,
			},
		},
		BillTo: &AuthorizeCIM.BillTo{
			FirstName:	 sub.Buyer.FirstName,
			LastName:	 sub.Buyer.LastName,
			Address:     sub.Buyer.BillingAddress.Line1,
			City:        sub.Buyer.BillingAddress.City,
			State:       sub.Buyer.BillingAddress.State,
			Zip:         sub.Buyer.BillingAddress.PostalCode,
			Country:     sub.Buyer.BillingAddress.Country,
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
					ExpirationDate: ToStringExpirationDate(pay.Account.Month, pay.Account.Year),
					CardCode:		pay.Account.CVC,
				},
				BillTo: &AuthorizeCIM.BillTo{
					Address:     pay.Buyer.BillingAddress.Line1,
					City:        pay.Buyer.BillingAddress.City,
					State:       pay.Buyer.BillingAddress.State,
					Zip:         pay.Buyer.BillingAddress.PostalCode,
					Country:     pay.Buyer.BillingAddress.Country,
				},
			}
	log.Warn("Payment %v", json2.Encode(pay), pay.Db.Context)
	log.Warn("New Transaction %v", json2.Encode(newTransaction), pay.Db.Context)
	return &newTransaction
}

func PaymentToPreviousTransaction(pay *payment.Payment) *AuthorizeCIM.PreviousTransaction{
	prevTransaction := AuthorizeCIM.PreviousTransaction{
				Amount: pay.Currency.ToStringNoSymbol(pay.Amount),
				RefId: pay.Account.TransId,
			}
	return &prevTransaction
}

func PopulatePaymentWithResponse(pay *payment.Payment, tran *AuthorizeCIM.TransactionResponse) (*payment.Payment, error) {
	msgs := make([]string, 0)
	for _, msg := range(tran.Response.Message.Message) {
		msgs = append(msgs, "Code: " + msg.Code + ", " + msg.Description)
	}

	errMsgs := make([]string, 0)
	for _, msg := range(tran.Response.Errors) {
		errMsgs = append(errMsgs, "Code: " + msg.ErrorCode + ", " + msg.ErrorText)
	}

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
	pay.Account.Messages = strings.Join(msgs, ", ")
	pay.Account.ErrorMessages = strings.Join(errMsgs, ", ")

	if len(errMsgs) > 0 {
		return pay, errors.New(pay.Account.ErrorMessages)
	}

	return pay, nil
}

func PopulateSubscriptionWithResponse(sub *order.Subscription, tran *AuthorizeCIM.SubscriptionResponse) *order.Subscription {
	if(tran.SubscriptionID != "") {
		sub.Ref.AuthorizeNet.SubscriptionId = tran.SubscriptionID
	}
	if(tran.Profile.CustomerProfileID != "") {
		sub.Ref.AuthorizeNet.CustomerProfileId = tran.Profile.CustomerProfileID
	}
	if(tran.Profile.CustomerPaymentProfileID != "") {
		sub.Ref.AuthorizeNet.CustomerPaymentProfileId = tran.Profile.CustomerPaymentProfileID
	}
	sub.Ref.Type = refs.AuthorizeNetRefType

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


func (c Client) NewSubscription(sub *order.Subscription) (*order.Subscription, error) {
	AuthorizeCIM.SetAPIInfo(c.loginId, c.transactionKey, c.getTestValue())
	AuthorizeCIM.SetHTTPClient(c.client)

	subscription := HanzoToAuthorizeSubscription(sub)

	response, err := ChargeSubscription(c.ctx, *subscription)

	if err != nil {
		log.Error("Authorize.net NewSubscription 1 %v, Error %v", json2.Encode(sub), err, c.ctx)
		return sub, err
	}

	if response.Approved() {
		sub, err = PopulateSubscriptionWithResponse(sub, response), nil
		if err != nil {
			log.Error("Authorize.net NewSubscription 2 %v", err, c.ctx)
		}
		sub.Status = order.ActiveSubscriptionStatus
		return sub, err
	} else {
		log.Warn("NewSubscription Failed")
		log.Debug("Authorize: Authorize.Net API did not approve transaction")
		return sub, NewSubscriptionFailedError
	}
}

// Update subscribe to a plan
func (c Client) UpdateSubscription(sub *order.Subscription) (*order.Subscription, error) {
	AuthorizeCIM.SetAPIInfo(c.loginId, c.transactionKey, c.getTestValue())
	AuthorizeCIM.SetHTTPClient(c.client)

	subscription := HanzoToAuthorizeSubscription(sub)
	subscription.SubscriptionId = sub.Ref.AuthorizeNet.SubscriptionId

	response, err := subscription.Update()

	if err != nil {
		log.Error("Authorize.net UpdateSubscription 1 %v, Error %v", json2.Encode(sub), err, c.ctx)
		return sub, err
	}

	if response.Approved() {
		sub, err = PopulateSubscriptionWithResponse(sub, response), nil
		if err != nil {
			log.Error("Authorize.net NewSubscription 2 %v", err, c.ctx)
		}
		return sub, err
	} else {
		log.Warn("UpdateSubscription Failed")
		log.Debug("Authorize: Authorize.Net API did not approve transaction")
		return sub, UpdateSubscriptionFailedError
	}
	return sub, err
}

// Subscribe to a plan
func (c Client) CancelSubscription(sub *order.Subscription) (*order.Subscription, error) {
	AuthorizeCIM.SetAPIInfo(c.loginId, c.transactionKey, c.getTestValue())
	AuthorizeCIM.SetHTTPClient(c.client)

	s := AuthorizeCIM.SetSubscription{
		Id: sub.Ref.AuthorizeNet.SubscriptionId,
	}
	_, err := s.Cancel()

	if err == nil {
		sub.Canceled = true
		sub.Status = order.CancelledSubscriptionStatus
		return sub, nil
	}
	return sub, err
}

// Do authorization, return token
func (c Client) Authorize(pay *payment.Payment) (*payment.Payment, error) {
	newTransaction := PaymentToNewTransaction(pay)

	log.Debug("Authorize: Setting API Info")
	AuthorizeCIM.SetAPIInfo(c.loginId, c.transactionKey, c.getTestValue())
	AuthorizeCIM.SetHTTPClient(c.client)

	log.JSON(newTransaction)

	log.Debug("Authorize: Invoking Authorize.net API")
	response, err := AuthOnly(c.ctx, *newTransaction)

	if err != nil {
		log.Error("Authorize.net Authorize 1 %v / %v, Error %v", json2.Encode(pay), json2.Encode(newTransaction), err, c.ctx)
		return pay, err
	}

	log.Debug("Authorize: Returned from Authorize.net API")
	if response.Approved() {
		log.Warn("Approved")
		pay, err := PopulatePaymentWithResponse(pay,response)
		if err != nil {
			log.Error("Authorize.net Authorize 2 %v", err, c.ctx)
		}
		return pay, err
	} else {
		log.Warn("Not Approved")
		log.Debug("Authorize: Authorize.Net API did not approve transaction")
		log.Debug("Authorize: Authorize.Net payment amount: %v", pay.Amount)
		log.Debug("Authorize: Authorize.Net card number: %v", pay.Account.Number)
		log.Debug("Authorize: Authorize.Net card expiration: %v", ToStringExpirationDate(pay.Account.Month, pay.Account.Year))
		log.Debug("Authorize: Authorize.Net returned error: %v", err, c.ctx)
		return pay, AuthorizeNotApprovedError
	}
}

/*func (c Client) AuthorizeSubscription(sub *subscription.Subscription) (*AuthorizeCIM.TransactionResponse, error) {

	t, err := c.API.Tokens.New(&stripe.TokenParams{
		Card: SubscriptionToCard(sub),
	})

	if err != nil {
		return nil, errors.New(err, c.ctx)
	}

	// Cast back to our token
	return (*Token)(t), nil
}*/

// Attempts to refund payment and updates the payment in datastore
func (c Client) RefundPayment(pay *payment.Payment, refundAmount currency.Cents) (*payment.Payment, error) {
	if refundAmount > pay.Amount {
		return pay, RefundGreaterThanPaymentError
	}

	if refundAmount+pay.AmountRefunded > pay.Amount {
		return pay, RefundGreaterThanPaymentError
	}

	if pay.Status == payment.Unpaid {
		return pay, UnableToRefundUnpaidTransactionError
	}

	AuthorizeCIM.SetAPIInfo(c.loginId, c.transactionKey, c.getTestValue())
	AuthorizeCIM.SetHTTPClient(c.client)
	newTransaction := PaymentToNewTransaction(pay)
	newTransaction.Amount = pay.Currency.ToStringNoSymbol(refundAmount)
	newTransaction.RefTransId = pay.Account.TransId
	var tr = AuthorizeCIM.TransactionRequest{
		TransactionType: "refundTransaction",
		Amount:          newTransaction.Amount,
		RefTransId:      newTransaction.RefTransId,
		Payment: &AuthorizeCIM.Payment{
			CreditCard: newTransaction.CreditCard,
		},
	}

	response, err := AuthorizeCIM.SendTransactionRequest(tr)

	if err != nil {
		return pay, err
	}

	log.JSON(response)

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
		log.Debug("Authorize: Authorize.Net API did not approve transaction")
		log.Debug("Authorize: Authorize.Net refTransId: %v", newTransaction.RefTransId)
		log.Debug("Authorize: Authorize.Net payment amount: %v", newTransaction.Amount)
		// log.Debug("Authorize: Authorize.Net card number: %v", newTransaction.CreditCard.CardNumber)
		// log.Debug("Authorize: Authorize.Net card expiration: %v", newTransaction.CreditCard.ExpirationDate)
		log.Debug("Authorize: Authorize.Net returned error: %v", err, c.ctx)

		if err == nil {
			err = MinimumRefundTimeNotReachedError
		}

		return pay, err
	}
}

/*func (c Client) GetCard(cardId string, customerId string) (*Card, error) {
	params := &stripe.CardParams{
		Customer: customerId,
	}

	card, err := c.API.Cards.Get(cardId, params)
	if err != nil {
		return nil, errors.New(err, c.ctx)
	}

	return (*Card)(card), nil
}

// Get Stripe customer
func (c Client) GetCustomer(token, usr *user.User) (*Customer, error) {
	params := &stripe.CustomerParams{}
	params.SetSource(token)

	cust, err := c.API.Customers.Get(usr.Accounts.Stripe.CustomerId, params)
	if err != nil {
		return nil, errors.New(err, c.ctx)
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
		return nil, errors.New(err, c.ctx)
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
		return nil, errors.New(err, c.ctx)
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
		return nil, errors.New(err, c.ctx)
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
		return nil, errors.New(err, c.ctx)
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
		return nil, errors.New(err, c.ctx)
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
		return nil, errors.New(err, c.ctx)
	}

	return (*Charge)(charge), nil
}*/

// Create new charge
func (c Client) Charge(pay *payment.Payment) (*payment.Payment, error) {

	newTransaction := PaymentToNewTransaction(pay)

	AuthorizeCIM.SetAPIInfo(c.loginId, c.transactionKey, c.getTestValue())
	AuthorizeCIM.SetHTTPClient(c.client)
	response, err := newTransaction.Charge()

	if err != nil {
		log.Error("Authorize.net Charge 1 %v", err, c.ctx)
		return pay, err
	}

	if response.Approved() {
		pay, err = PopulatePaymentWithResponse(pay,response)
		if err != nil {
			log.Error("Authorize.net Charge 2 %v", err, c.ctx)
		} else {
			pay.Captured = true
		}
		return pay, err
	} else {
		return pay, ChargeNotApprovedError
	}
}

// Capture charge
func (c Client) Capture(pay *payment.Payment) (*payment.Payment, error) {
	log.Debug("Capture charge '%s'", pay.Account.AuthCode)
	oldTransaction := PaymentToPreviousTransaction(pay)

	AuthorizeCIM.SetAPIInfo(c.loginId, c.transactionKey, c.getTestValue())
	AuthorizeCIM.SetHTTPClient(c.client)
	response, err := Capture(c.ctx, *oldTransaction)

	if err != nil {
		log.Error("Authorize.net Capture 1 %v", err, c.ctx)
		return pay, err
	}

	if response.Approved() {
		pay, err = PopulatePaymentWithResponse(pay,response)
		if err != nil {
			log.Error("Authorize.net Capture 2 %v", err, c.ctx)
		} else {
			pay.Captured = true
		}
		return pay, err
	} else {
		return pay, CaptureNotApprovedError
	}
}

func AuthOnly(ctx context.Context, tranx AuthorizeCIM.NewTransaction) (*AuthorizeCIM.TransactionResponse, error) {
	var new AuthorizeCIM.TransactionRequest
	new = AuthorizeCIM.TransactionRequest{
		TransactionType: "authOnlyTransaction",
		Amount:          tranx.Amount,
		Payment: &AuthorizeCIM.Payment{
			CreditCard: tranx.CreditCard,
		},
	}
	response, err := SendTransactionRequest(ctx, new)
	return response, err
}

func Capture(ctx context.Context, tranx AuthorizeCIM.PreviousTransaction) (*AuthorizeCIM.TransactionResponse, error) {
	var new AuthorizeCIM.TransactionRequest
	new = AuthorizeCIM.TransactionRequest{
		TransactionType: "priorAuthCaptureTransaction",
		RefTransId:      tranx.RefId,
	}
	response, err := SendTransactionRequest(ctx, new)
	return response, err
}

func SendTransactionRequest(ctx context.Context, input AuthorizeCIM.TransactionRequest) (*AuthorizeCIM.TransactionResponse, error) {
	action := AuthorizeCIM.CreatePayment{
		CreateTransactionRequest: AuthorizeCIM.CreateTransactionRequest{
			MerchantAuthentication: AuthorizeCIM.GetAuthentication(),
			TransactionRequest:     input,
		},
	}

	jsoned, err := json.Marshal(action)
	if err != nil {
		return nil, err
	}

	response, err := SendRequest(ctx, jsoned)
	if err != nil {
		return nil, err
	}

	var dat AuthorizeCIM.TransactionResponse

	log.Warn("Returned Data: %s",response, ctx)
	err = json.Unmarshal(response, &dat)
	if err != nil {
		return nil, err
	}
	return &dat, err
}

func SendRequest(ctx context.Context, input []byte) ([]byte, error) {
	api_endpoint := "https://apitest.authorize.net/xml/v1/request.api"
	req, err := http.NewRequest("POST", api_endpoint, bytes.NewBuffer(input))
	req.Header.Set("Content-Type", "application/json")

	client := urlfetch.Client(ctx)
	client.Transport = &urlfetch.Transport{
		Context: ctx,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}

	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}
	log.Warn("Request %s", string(input), ctx)

	dump, _ := httputil.DumpResponse(resp, true)
	log.Warn("Response %s", string(dump), ctx)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))
	return body, err
}

func ChargeSubscription(ctx context.Context, sub AuthorizeCIM.Subscription) (*AuthorizeCIM.SubscriptionResponse, error) {
	return SendSubscription(ctx, sub)
}

func SendSubscription(ctx context.Context, sub AuthorizeCIM.Subscription) (*AuthorizeCIM.SubscriptionResponse, error) {
	action := AuthorizeCIM.CreateSubscriptionRequest{
		ARBCreateSubscriptionRequest: AuthorizeCIM.ARBCreateSubscriptionRequest{
			MerchantAuthentication: AuthorizeCIM.GetAuthentication(),
			Subscription:           sub,
		},
	}

	jsoned, err := json.Marshal(action)
	if err != nil {
		return nil, err
	}

	response, err := SendRequest(ctx, jsoned)
	if err != nil {
		return nil, err
	}

	var dat AuthorizeCIM.SubscriptionResponse
	err = json.Unmarshal(response, &dat)
	if err != nil {
		return nil, err
	}
	return &dat, err
}
