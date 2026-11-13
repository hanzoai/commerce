// Package stripe implements the Stripe payment processor for Commerce.
// Uses the Stripe REST API directly (no SDK dependency), consistent with
// other provider implementations in this codebase.
package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/money"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

const (
	baseURL        = "https://api.stripe.com/v1"
	defaultTimeout = 30 * time.Second
	apiVersion     = "2024-12-18.acacia"
)

// Config holds Stripe API credentials.
type Config struct {
	SecretKey      string
	PublishableKey string
	WebhookSecret  string
}

// Provider implements PaymentProcessor, SubscriptionProcessor, and CustomerProcessor.
type Provider struct {
	*processor.BaseProcessor
	secretKey      string
	publishableKey string
	webhookSecret  string
	client         *http.Client
}

// NewProvider creates a configured Stripe provider instance.
func NewProvider(cfg Config) *Provider {
	p := &Provider{
		BaseProcessor:  processor.NewBaseProcessor(processor.Stripe, supportedCurrencies()),
		secretKey:      cfg.SecretKey,
		publishableKey: cfg.PublishableKey,
		webhookSecret:  cfg.WebhookSecret,
		client:         &http.Client{Timeout: defaultTimeout},
	}
	if cfg.SecretKey != "" {
		p.SetConfigured(true)
	}
	return p
}

func init() {
	sk := os.Getenv("STRIPE_SECRET_KEY")
	whs := os.Getenv("STRIPE_WEBHOOK_SECRET")
	pk := os.Getenv("STRIPE_PUBLISHABLE_KEY")

	p := &Provider{
		BaseProcessor:  processor.NewBaseProcessor(processor.Stripe, supportedCurrencies()),
		secretKey:      sk,
		publishableKey: pk,
		webhookSecret:  whs,
		client:         &http.Client{Timeout: defaultTimeout},
	}

	if sk != "" {
		p.SetConfigured(true)
	}

	processor.Register(p)
}

func supportedCurrencies() []currency.Type {
	return []currency.Type{
		currency.USD, currency.EUR, currency.GBP, currency.CAD, currency.AUD,
		currency.JPY, currency.CHF, currency.NZD, "sgd", "hkd",
		"nok", "sek", "dkk", "pln", "brl", "mxn",
		"czk", "huf", "ron", "bgn", "inr", "krw", "cny", "zar",
	}
}

// Type returns the processor type.
func (p *Provider) Type() processor.ProcessorType {
	return processor.Stripe
}

// IsAvailable checks if the processor is configured.
func (p *Provider) IsAvailable(ctx context.Context) bool {
	return p.secretKey != ""
}

// SupportedCurrencies returns currencies this processor supports.
func (p *Provider) SupportedCurrencies() []currency.Type {
	return supportedCurrencies()
}

// ---------------------------------------------------------------------------
// PaymentProcessor
// ---------------------------------------------------------------------------

// Charge creates a PaymentIntent and confirms it immediately, with redirects off.
//
// req.Token is a PaymentMethod ("pm_…", a card the browser's Stripe.js minted or
// one on file with req.CustomerID), or a PaymentIntent ("pi_…") the buyer already
// authenticated in the browser: the intent is then COMPLETED, not created — see
// completeIntent. req.Options["offSession"] marks a charge made without the buyer
// present (a renewal, an auto-reload).
//
// A card that needs its bank's authentication moves no money and is not a paid
// charge: the result is failed, with an *processor.ActionRequired carrying what
// the browser needs to run the step.
func (p *Provider) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}
	if strings.HasPrefix(req.Token, "pi_") {
		return p.completeIntent(ctx, req)
	}
	// The card's own country, read from the card: a buyer cannot choose it.
	if strings.HasPrefix(req.Token, "pm_") {
		card, err := p.card(ctx, req.Token)
		if err != nil {
			return &processor.PaymentResult{Success: false, ErrorMessage: err.Error(), Error: err}, err
		}
		if processor.Sanctioned(card.Country) {
			return &processor.PaymentResult{Success: false, ErrorMessage: errSanctioned.Error(), Error: errSanctioned}, errSanctioned
		}
	}

	params := url.Values{}
	params.Set("amount", strconv.FormatInt(int64(req.Amount), 10))
	params.Set("currency", strings.ToLower(string(req.Currency)))
	params.Set("confirm", "true")
	params.Set("automatic_payment_methods[enabled]", "true")
	params.Set("automatic_payment_methods[allow_redirects]", "never")

	if req.Token != "" {
		params.Set("payment_method", req.Token)
	}
	if req.CustomerID != "" {
		params.Set("customer", req.CustomerID)
	}
	if off, _ := req.Options["offSession"].(bool); off {
		params.Set("off_session", "true")
	}
	if req.Description != "" {
		params.Set("description", req.Description)
	}
	if req.OrderID != "" {
		params.Set("metadata[order_id]", req.OrderID)
	}
	for k, v := range req.Metadata {
		params.Set("metadata["+k+"]", fmt.Sprintf("%v", v))
	}

	var pi paymentIntent
	if err := p.postKey(ctx, "/payment_intents", params, req.IdempotencyKey, &pi); err != nil {
		// A card Stripe refuses for authentication carries the intent that waits
		// for it; that is the buyer's step, not a decline.
		if pe := intentOf(err); pe != nil && pe.Status == "requires_action" {
			return actionRequired(pe)
		}
		return &processor.PaymentResult{
			Success:      false,
			ErrorMessage: err.Error(),
			Error:        err,
		}, err
	}
	return resultOf(&pi)
}

// completeIntent finishes a PaymentIntent the buyer authenticated in the browser
// (Stripe.js handleNextAction / confirmCardPayment). It takes nothing: it reads
// the intent and says whether it SUCCEEDED, for the amount, currency and buyer
// this request names — an intent is a bearer token for its own payment, so one
// made for another amount or another buyer is refused.
func (p *Provider) completeIntent(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	var pi paymentIntent
	if err := p.get(ctx, "/payment_intents/"+url.PathEscape(req.Token), nil, &pi); err != nil {
		return &processor.PaymentResult{Success: false, ErrorMessage: err.Error(), Error: err}, err
	}
	if pi.PaymentMethod != "" {
		card, err := p.card(ctx, pi.PaymentMethod)
		if err != nil {
			return &processor.PaymentResult{Success: false, ErrorMessage: err.Error(), Error: err}, err
		}
		if processor.Sanctioned(card.Country) {
			return &processor.PaymentResult{Success: false, ErrorMessage: errSanctioned.Error(), Error: errSanctioned}, errSanctioned
		}
	}
	// A payment already refunded, in whole or part, is not money in hand.
	if pi.LatestCharge != "" {
		ok, err := p.ChargeSound(ctx, pi.LatestCharge, pi.Amount)
		if err != nil {
			return &processor.PaymentResult{Success: false, ErrorMessage: err.Error(), Error: err}, err
		}
		if !ok {
			return failed(&pi, "this payment was refunded, disputed or not received in full")
		}
	}
	switch {
	case pi.Amount != int64(req.Amount) || !strings.EqualFold(pi.Currency, string(req.Currency)):
		return failed(&pi, "this payment was made for a different amount")
	case req.Metadata != nil && fmt.Sprint(pi.Metadata["hanzo_subject"]) != fmt.Sprint(req.Metadata["hanzo_subject"]):
		return failed(&pi, "this payment was made for a different account")
	}
	return resultOf(&pi)
}

// resultOf is a PaymentIntent as a charge result: paid only when it SUCCEEDED.
func resultOf(pi *paymentIntent) (*processor.PaymentResult, error) {
	switch pi.Status {
	case "succeeded":
		return &processor.PaymentResult{
			Success:       true,
			TransactionID: pi.ID,
			ProcessorRef:  pi.LatestCharge,
			Status:        pi.Status,
		}, nil
	case "requires_action":
		return actionRequired(pi)
	default:
		// processing, requires_payment_method, canceled…: not money in hand. A
		// "processing" intent that settles later is reconciled by hand: its webhook
		// finds no receipt, so it credits nothing and raises the alarm.
		return failed(pi, "the payment did not complete: "+pi.Status)
	}
}

func actionRequired(pi *paymentIntent) (*processor.PaymentResult, error) {
	err := &processor.ActionRequired{ID: pi.ID, ClientSecret: pi.ClientSecret}
	return &processor.PaymentResult{
		Success:       false,
		TransactionID: pi.ID,
		ProcessorRef:  pi.ID,
		Status:        "requires_action",
		ErrorMessage:  err.Error(),
		Error:         err,
	}, err
}

func failed(pi *paymentIntent, why string) (*processor.PaymentResult, error) {
	err := processor.NewPaymentError(processor.Stripe, "payment_not_completed", why, nil)
	return &processor.PaymentResult{
		Success:       false,
		TransactionID: pi.ID,
		ProcessorRef:  pi.ID,
		Status:        pi.Status,
		ErrorMessage:  why,
		Error:         &processor.Answer{Err: err},
	}, &processor.Answer{Err: err}
}

// Authorize creates a PaymentIntent with capture_method=manual.
func (p *Provider) Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("amount", strconv.FormatInt(int64(req.Amount), 10))
	params.Set("currency", strings.ToLower(string(req.Currency)))
	params.Set("capture_method", "manual")
	params.Set("confirm", "true")
	params.Set("automatic_payment_methods[enabled]", "true")
	params.Set("automatic_payment_methods[allow_redirects]", "never")

	if req.Token != "" {
		params.Set("payment_method", req.Token)
	}
	if req.CustomerID != "" {
		params.Set("customer", req.CustomerID)
	}
	if req.Description != "" {
		params.Set("description", req.Description)
	}
	if req.OrderID != "" {
		params.Set("metadata[order_id]", req.OrderID)
	}

	var pi paymentIntent
	if err := p.postKey(ctx, "/payment_intents", params, req.IdempotencyKey, &pi); err != nil {
		return &processor.PaymentResult{
			Success:      false,
			ErrorMessage: err.Error(),
			Error:        err,
		}, err
	}

	return &processor.PaymentResult{
		Success:       pi.Status == "requires_capture",
		TransactionID: pi.ID,
		ProcessorRef:  pi.ID,
		Status:        "authorized",
	}, nil
}

// Capture captures a previously authorized PaymentIntent.
func (p *Provider) Capture(ctx context.Context, transactionID string, amount money.Amount) (*processor.PaymentResult, error) {
	params := url.Values{}
	if amount.Sign() > 0 {
		params.Set("amount_to_capture", strconv.FormatInt(amount.Minor().Int64(), 10))
	}

	var pi paymentIntent
	if err := p.post(ctx, "/payment_intents/"+transactionID+"/capture", params, &pi); err != nil {
		return &processor.PaymentResult{
			Success:      false,
			ErrorMessage: err.Error(),
			Error:        err,
		}, err
	}

	return &processor.PaymentResult{
		Success:       pi.Status == "succeeded",
		TransactionID: pi.ID,
		ProcessorRef:  pi.LatestCharge,
		Status:        "captured",
	}, nil
}

// Refund creates a refund on a PaymentIntent's charge.
func (p *Provider) Refund(ctx context.Context, req processor.RefundRequest) (*processor.RefundResult, error) {
	params := url.Values{}
	params.Set("payment_intent", req.TransactionID)

	if req.Amount.Sign() > 0 {
		params.Set("amount", strconv.FormatInt(req.Amount.Minor().Int64(), 10))
	}
	if req.Reason != "" {
		params.Set("reason", mapRefundReason(req.Reason))
	}
	for k, v := range req.Metadata {
		params.Set("metadata["+k+"]", fmt.Sprintf("%v", v))
	}

	var ref refund
	if err := p.postKey(ctx, "/refunds", params, req.IdempotencyKey, &ref); err != nil {
		return &processor.RefundResult{
			Success:      false,
			ErrorMessage: err.Error(),
			Error:        err,
		}, err
	}

	return &processor.RefundResult{
		Success:      ref.Status == "succeeded" || ref.Status == "pending",
		RefundID:     ref.ID,
		ProcessorRef: ref.ID,
	}, nil
}

// GetTransaction retrieves a PaymentIntent by ID.
func (p *Provider) GetTransaction(ctx context.Context, txID string) (*processor.Transaction, error) {
	var pi paymentIntent
	if err := p.get(ctx, "/payment_intents/"+txID, nil, &pi); err != nil {
		return nil, err
	}

	return &processor.Transaction{
		ID:           pi.ID,
		ProcessorRef: pi.LatestCharge,
		Type:         "charge",
		Amount:       currency.Cents(pi.Amount),
		Currency:     currency.Type(pi.Currency),
		Status:       pi.Status,
		CustomerID:   pi.Customer,
		CreatedAt:    pi.Created,
		UpdatedAt:    pi.Created,
		Metadata:     withSecret(withPaymentMethod(pi.Metadata, pi.PaymentMethod), pi.ClientSecret),
	}, nil
}

// ValidateWebhook verifies a Stripe webhook signature and parses the event.
func (p *Provider) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*processor.WebhookEvent, error) {
	if p.webhookSecret == "" {
		return nil, processor.ErrWebhookValidationFailed
	}

	// Parse Stripe-Signature header: t=...,v1=...
	parts := parseSignatureHeader(signature)
	timestamp := parts["t"]
	if timestamp == "" || len(v1Sigs(signature)) == 0 {
		return nil, processor.ErrWebhookValidationFailed
	}

	// Verify signature: HMAC-SHA256(secret, "timestamp.payload")
	signedPayload := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(p.webhookSecret))
	mac.Write([]byte(signedPayload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	// Stripe signs with every secret it holds for the endpoint while one is being
	// rolled: any v1 that matches admits the delivery.
	matched := false
	for _, v1Sig := range v1Sigs(signature) {
		if hmac.Equal([]byte(v1Sig), []byte(expectedSig)) {
			matched = true
		}
	}
	if !matched {
		return nil, processor.ErrWebhookValidationFailed
	}

	// Check timestamp tolerance (5 minutes)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil, processor.ErrWebhookValidationFailed
	}
	if time.Since(time.Unix(ts, 0)) > 5*time.Minute {
		return nil, processor.ErrWebhookValidationFailed
	}

	// Parse event
	var evt stripeEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return nil, fmt.Errorf("failed to parse webhook event: %w", err)
	}

	// The event's own time orders reports of one object; the signature's guards
	// replay. The event says which kind of money it concerns, so a test-mode event
	// is booked as test money whatever deployment it reaches.
	at := evt.Created
	if at <= 0 {
		at = ts
	}
	if evt.Data.Object == nil {
		evt.Data.Object = map[string]interface{}{}
	}
	evt.Data.Object["stripe_livemode"] = evt.Livemode
	return &processor.WebhookEvent{
		ID:        evt.ID,
		Type:      mapEventType(evt.Type),
		Processor: processor.Stripe,
		Data:      evt.Data.Object,
		Timestamp: at,
	}, nil
}

// ---------------------------------------------------------------------------
// SubscriptionProcessor
// ---------------------------------------------------------------------------

// CreateSubscription creates a Stripe subscription.
func (p *Provider) CreateSubscription(ctx context.Context, req processor.SubscriptionRequest) (*processor.Subscription, error) {
	params := url.Values{}
	params.Set("customer", req.CustomerID)
	params.Set("items[0][price]", req.PlanID)

	if req.Quantity > 0 {
		params.Set("items[0][quantity]", strconv.Itoa(req.Quantity))
	}
	if req.TrialDays > 0 {
		params.Set("trial_period_days", strconv.Itoa(req.TrialDays))
	}
	if req.PaymentToken != "" {
		params.Set("default_payment_method", req.PaymentToken)
	}
	for k, v := range req.Metadata {
		params.Set("metadata["+k+"]", fmt.Sprintf("%v", v))
	}

	var sub stripeSub
	if err := p.post(ctx, "/subscriptions", params, &sub); err != nil {
		return nil, err
	}

	return mapSubscription(&sub), nil
}

// GetSubscription retrieves a subscription.
func (p *Provider) GetSubscription(ctx context.Context, subscriptionID string) (*processor.Subscription, error) {
	var sub stripeSub
	if err := p.get(ctx, "/subscriptions/"+subscriptionID, nil, &sub); err != nil {
		return nil, err
	}
	return mapSubscription(&sub), nil
}

// CancelSubscription cancels a subscription.
func (p *Provider) CancelSubscription(ctx context.Context, subscriptionID string, immediately bool) error {
	if immediately {
		return p.del(ctx, "/subscriptions/"+subscriptionID, nil)
	}
	params := url.Values{}
	params.Set("cancel_at_period_end", "true")
	var sub stripeSub
	return p.post(ctx, "/subscriptions/"+subscriptionID, params, &sub)
}

// UpdateSubscription modifies a subscription.
func (p *Provider) UpdateSubscription(ctx context.Context, subscriptionID string, req processor.SubscriptionUpdate) (*processor.Subscription, error) {
	params := url.Values{}
	if req.PlanID != "" {
		// Retrieve current sub to get the item ID
		var current stripeSub
		if err := p.get(ctx, "/subscriptions/"+subscriptionID, nil, &current); err != nil {
			return nil, err
		}
		if len(current.Items.Data) > 0 {
			params.Set("items[0][id]", current.Items.Data[0].ID)
			params.Set("items[0][price]", req.PlanID)
		}
	}
	if req.Quantity > 0 {
		params.Set("items[0][quantity]", strconv.Itoa(req.Quantity))
	}
	if req.CancelAtPeriodEnd != nil {
		params.Set("cancel_at_period_end", strconv.FormatBool(*req.CancelAtPeriodEnd))
	}

	var sub stripeSub
	if err := p.post(ctx, "/subscriptions/"+subscriptionID, params, &sub); err != nil {
		return nil, err
	}
	return mapSubscription(&sub), nil
}

// ListSubscriptions lists subscriptions for a customer.
func (p *Provider) ListSubscriptions(ctx context.Context, customerID string) ([]*processor.Subscription, error) {
	params := url.Values{}
	params.Set("customer", customerID)
	params.Set("limit", "100")

	var list struct {
		Data []stripeSub `json:"data"`
	}
	if err := p.get(ctx, "/subscriptions", params, &list); err != nil {
		return nil, err
	}

	result := make([]*processor.Subscription, len(list.Data))
	for i := range list.Data {
		result[i] = mapSubscription(&list.Data[i])
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// CustomerProcessor
// ---------------------------------------------------------------------------

// CreateCustomer creates a Stripe customer.
func (p *Provider) CreateCustomer(ctx context.Context, email, name string, metadata map[string]interface{}) (string, error) {
	params := url.Values{}
	if email != "" {
		params.Set("email", email)
	}
	if name != "" {
		params.Set("name", name)
	}
	for k, v := range metadata {
		params.Set("metadata["+k+"]", fmt.Sprintf("%v", v))
	}

	// One customer per reference, whatever retries: Stripe answers a repeated
	// create under the same key with the customer already made.
	key := ""
	if ref, _ := metadata["reference"].(string); ref != "" {
		key = "customer:" + ref
	}
	var cust stripeCustomer
	if err := p.postKey(ctx, "/customers", params, key, &cust); err != nil {
		return "", err
	}
	return cust.ID, nil
}

// GetCustomer retrieves customer details.
func (p *Provider) GetCustomer(ctx context.Context, customerID string) (map[string]interface{}, error) {
	var cust stripeCustomer
	if err := p.get(ctx, "/customers/"+customerID, nil, &cust); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":       cust.ID,
		"email":    cust.Email,
		"name":     cust.Name,
		"created":  cust.Created,
		"metadata": cust.Metadata,
	}, nil
}

// UpdateCustomer updates customer details.
func (p *Provider) UpdateCustomer(ctx context.Context, customerID string, updates map[string]interface{}) error {
	params := url.Values{}
	if email, ok := updates["email"].(string); ok {
		params.Set("email", email)
	}
	if name, ok := updates["name"].(string); ok {
		params.Set("name", name)
	}
	if meta, ok := updates["metadata"].(map[string]interface{}); ok {
		for k, v := range meta {
			params.Set("metadata["+k+"]", fmt.Sprintf("%v", v))
		}
	}
	var cust stripeCustomer
	return p.post(ctx, "/customers/"+customerID, params, &cust)
}

// DeleteCustomer removes a customer.
func (p *Provider) DeleteCustomer(ctx context.Context, customerID string) error {
	return p.del(ctx, "/customers/"+customerID, nil)
}

// AddPaymentMethod attaches a payment method to a customer.
func (p *Provider) AddPaymentMethod(ctx context.Context, customerID, token string) (string, error) {
	params := url.Values{}
	params.Set("customer", customerID)

	var pm struct {
		ID string `json:"id"`
	}
	if err := p.post(ctx, "/payment_methods/"+token+"/attach", params, &pm); err != nil {
		return "", err
	}
	return pm.ID, nil
}

// RemovePaymentMethod detaches a payment method from a customer.
func (p *Provider) RemovePaymentMethod(ctx context.Context, customerID, paymentMethodID string) error {
	var pm struct {
		ID string `json:"id"`
	}
	return p.post(ctx, "/payment_methods/"+paymentMethodID+"/detach", url.Values{}, &pm)
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func (p *Provider) post(ctx context.Context, path string, params url.Values, result interface{}) error {
	return p.postKey(ctx, path, params, "", result)
}

// postKey is post carrying an Idempotency-Key, so Stripe de-dups a retried money
// move at Stripe itself — the backstop every card money move in this service
// relies on (processor.PaymentRequest.IdempotencyKey). An empty key sends none.
func (p *Provider) postKey(ctx context.Context, path string, params url.Values, key string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	if key = strings.TrimSpace(key); key != "" {
		req.Header.Set("Idempotency-Key", fitKey(key))
	}
	return p.doRequest(req, result)
}

func (p *Provider) get(ctx context.Context, path string, params url.Values, result interface{}) error {
	u := baseURL + path
	if params != nil && len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return p.doRequest(req, result)
}

func (p *Provider) del(ctx context.Context, path string, params url.Values) error {
	u := baseURL + path
	if params != nil && len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	var discard json.RawMessage
	return p.doRequest(req, &discard)
}

func (p *Provider) doRequest(req *http.Request, result interface{}) error {
	req.SetBasicAuth(p.secretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Stripe-Version", apiVersion)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("stripe request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("stripe read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr stripeAPIError
		var err error
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
			err = processor.NewPaymentError(processor.Stripe, apiErr.Error.Code, apiErr.Error.Message, nil)
			if in := apiErr.Error.PaymentIntent; in != nil {
				err = &intentError{error: err, intent: in}
			}
		} else {
			err = fmt.Errorf("stripe API error (HTTP %d): %s", resp.StatusCode, string(body))
		}
		// A 4xx is Stripe's own refusal: the call reached it, it said no, and nothing
		// was done. (409 is an idempotency race and 429 a rate limit: no answer yet.)
		if resp.StatusCode < 500 && resp.StatusCode != http.StatusConflict && resp.StatusCode != http.StatusTooManyRequests {
			return &processor.Answer{Err: err}
		}
		return err
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("stripe decode response: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Stripe API types
// ---------------------------------------------------------------------------

type paymentIntent struct {
	ID            string                 `json:"id"`
	Status        string                 `json:"status"`
	Amount        int64                  `json:"amount"`
	Currency      string                 `json:"currency"`
	Customer      string                 `json:"customer"`
	LatestCharge  string                 `json:"latest_charge"`
	ClientSecret  string                 `json:"client_secret"`
	PaymentMethod string                 `json:"payment_method"`
	Created       int64                  `json:"created"`
	Metadata      map[string]interface{} `json:"metadata"`
}

type refund struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Amount int64  `json:"amount"`
}

type stripeSub struct {
	ID                 string                 `json:"id"`
	Status             string                 `json:"status"`
	Customer           string                 `json:"customer"`
	CurrentPeriodStart int64                  `json:"current_period_start"`
	CurrentPeriodEnd   int64                  `json:"current_period_end"`
	CancelAtPeriodEnd  bool                   `json:"cancel_at_period_end"`
	Metadata           map[string]interface{} `json:"metadata"`
	Items              struct {
		Data []struct {
			ID    string `json:"id"`
			Price struct {
				ID string `json:"id"`
			} `json:"price"`
			Quantity int `json:"quantity"`
		} `json:"data"`
	} `json:"items"`
}

type stripeCustomer struct {
	ID       string                 `json:"id"`
	Email    string                 `json:"email"`
	Name     string                 `json:"name"`
	Created  int64                  `json:"created"`
	Metadata map[string]interface{} `json:"metadata"`
}

type stripeEvent struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Created  int64  `json:"created"`
	Livemode bool   `json:"livemode"`
	Data     struct {
		Object map[string]interface{} `json:"object"`
	} `json:"data"`
}

type stripeAPIError struct {
	Error struct {
		Type          string         `json:"type"`
		Code          string         `json:"code"`
		Message       string         `json:"message"`
		PaymentIntent *paymentIntent `json:"payment_intent"`
	} `json:"error"`
}

// intentError is a Stripe error that names the PaymentIntent it concerns.
type intentError struct {
	error
	intent *paymentIntent
}

func (e *intentError) Unwrap() error { return e.error }

// intentOf is the PaymentIntent a Stripe error names, or nil.
func intentOf(err error) *paymentIntent {
	var ie *intentError
	if errors.As(err, &ie) {
		return ie.intent
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mapSubscription(sub *stripeSub) *processor.Subscription {
	planID := ""
	if len(sub.Items.Data) > 0 {
		planID = sub.Items.Data[0].Price.ID
	}
	return &processor.Subscription{
		ID:                 sub.ID,
		CustomerID:         sub.Customer,
		PlanID:             planID,
		Status:             sub.Status,
		CurrentPeriodStart: sub.CurrentPeriodStart,
		CurrentPeriodEnd:   sub.CurrentPeriodEnd,
		CancelAtPeriodEnd:  sub.CancelAtPeriodEnd,
		Metadata:           sub.Metadata,
	}
}

// v1Sigs is every v1 signature a Stripe-Signature header carries.
func v1Sigs(header string) []string {
	var out []string
	for _, part := range strings.Split(header, ",") {
		if k, v, ok := strings.Cut(strings.TrimSpace(part), "="); ok && k == "v1" {
			out = append(out, v)
		}
	}
	return out
}

func parseSignatureHeader(header string) map[string]string {
	result := make(map[string]string)
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return result
}

func mapRefundReason(reason string) string {
	switch strings.ToLower(reason) {
	case "duplicate":
		return "duplicate"
	case "fraudulent", "fraud":
		return "fraudulent"
	default:
		return "requested_by_customer"
	}
}

func mapEventType(stripeType string) string {
	switch stripeType {
	case "payment_intent.succeeded":
		return "payment.completed"
	case "payment_intent.payment_failed":
		return "payment.failed"
	case "payment_intent.processing":
		return "payment.pending"
	case "payment_intent.canceled":
		return "payment.canceled"
	case "charge.refunded":
		return "refund.succeeded"
	case "charge.refund.updated":
		return "refund.updated"
	case "charge.dispute.created":
		return "dispute.created"
	case "charge.dispute.updated", "charge.dispute.closed", "charge.dispute.funds_withdrawn", "charge.dispute.funds_reinstated":
		return "dispute.updated"
	case "customer.subscription.created":
		return "subscription.created"
	case "customer.subscription.updated":
		return "subscription.updated"
	case "customer.subscription.deleted":
		return "subscription.canceled"
	case "invoice.paid":
		return "invoice.paid"
	case "invoice.payment_failed":
		return "invoice.payment_failed"
	case "customer.created":
		return "customer.created"
	case "customer.updated":
		return "customer.updated"
	case "customer.deleted":
		return "customer.deleted"
	default:
		return stripeType
	}
}

// Compile-time interface checks.
var (
	_ processor.PaymentProcessor      = (*Provider)(nil)
	_ processor.SubscriptionProcessor = (*Provider)(nil)
	_ processor.CustomerProcessor     = (*Provider)(nil)
)

// ReceivesWebhooks reports whether this provider holds the secret Stripe signs
// its deliveries with (STRIPE_WEBHOOK_SECRET). Without it no delivery is admitted.
func (p *Provider) ReceivesWebhooks() bool { return p.webhookSecret != "" }

// Reaches reports whether Stripe takes a payment by method from a buyer in
// country. Stripe's card form takes a typed card, Apple Pay and Google Pay;
// Charge confirms with redirects off, so a redirect method (iDEAL, Bancontact,
// ...) and a delayed one (ACH, SEPA debit) are not taken here. No country is
// refused: nothing in this service records where Stripe declines a buyer, so
// none is guessed at.
func (p *Provider) Reaches(country, method string) bool {
	switch method {
	case "", "card", "apple_pay", "google_pay":
		return true
	default:
		return false
	}
}

var (
	_ processor.Reach           = (*Provider)(nil)
	_ processor.WebhookReceiver = (*Provider)(nil)
)

// withPaymentMethod is the intent's metadata with the card it names, so a caller
// that holds only the intent's id can find the saved card it was charged on.
func withPaymentMethod(meta map[string]interface{}, pm string) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range meta {
		out[k] = v
	}
	if pm != "" {
		out["payment_method"] = pm
	}
	return out
}

// ChargeSound says whether a charge is money in hand for amount: captured in full,
// not refunded in whole or part, and not disputed. A caller that is about to
// credit one asks first.
func (p *Provider) ChargeSound(ctx context.Context, chargeID string, amount int64) (bool, error) {
	var ch struct {
		Refunded       bool  `json:"refunded"`
		Disputed       bool  `json:"disputed"`
		AmountRefunded int64 `json:"amount_refunded"`
		AmountCaptured int64 `json:"amount_captured"`
	}
	if err := p.get(ctx, "/charges/"+url.PathEscape(chargeID), nil, &ch); err != nil {
		return false, err
	}
	if ch.Refunded || ch.Disputed || ch.AmountRefunded > 0 || (amount > 0 && ch.AmountCaptured < amount) {
		return false, nil
	}
	return true, nil
}

// withSecret adds the intent's client secret, which only a caller that is about
// to hand it to the buyer's own browser may use.
func withSecret(meta map[string]interface{}, secret string) map[string]interface{} {
	if secret != "" {
		meta["client_secret"] = secret
	}
	return meta
}

// CancelIntent cancels a PaymentIntent that waits on its buyer, so it can never
// succeed after commerce has stopped waiting for it.
func (p *Provider) CancelIntent(ctx context.Context, id string) error {
	var pi paymentIntent
	return p.post(ctx, "/payment_intents/"+url.PathEscape(id)+"/cancel", url.Values{"cancellation_reason": {"abandoned"}}, &pi)
}

// ChargeRefunds lists the refunds made on a charge, newest first.
func (p *Provider) ChargeRefunds(ctx context.Context, chargeID string) ([]map[string]interface{}, error) {
	var list struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := p.get(ctx, "/refunds", url.Values{"charge": {chargeID}, "limit": {"100"}}, &list); err != nil {
		return nil, err
	}
	return list.Data, nil
}
