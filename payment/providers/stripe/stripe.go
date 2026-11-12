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

// Charge creates a PaymentIntent and confirms it, or reads back the one the
// buyer has just authenticated.
//
// The options a caller passes in req.Options (all optional):
//
//   - "off_session": true — the buyer is not there (a renewal, a dunning retry,
//     an automatic reload). Stripe is told so, and a card whose bank insists on
//     authentication is a decline (authentication_required): nobody is present
//     to authenticate, and the payer is asked to update the card instead.
//   - "save": true — the buyer is there and the card will be charged again off
//     session: setup_future_usage=off_session, so Stripe collects the mandate
//     and SCA exemptions a later off-session charge needs.
//   - "payment_intent": "pi_…" — the intent this order already started, which
//     the buyer then authenticated in the page. It is read, never created again,
//     and it must be this order's: its amount, currency, customer, payment
//     method and the order's idempotency key (stamped as metadata on creation)
//     all agree, or it is refused.
//
// A buyer-present charge whose bank asks for authentication answers
// *processor.ActionRequired with the client secret the page needs; no money
// has moved.
func (p *Provider) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}
	if id := optString(req.Options, "payment_intent"); id != "" {
		return p.resume(ctx, req, id)
	}
	off := optBool(req.Options, "off_session")

	params := url.Values{}
	params.Set("amount", strconv.FormatInt(int64(req.Amount), 10))
	params.Set("currency", strings.ToLower(string(req.Currency)))
	params.Set("confirm", "true")
	params.Set("automatic_payment_methods[enabled]", "true")
	params.Set("automatic_payment_methods[allow_redirects]", "never")
	if off {
		params.Set("off_session", "true")
	} else if optBool(req.Options, "save") {
		params.Set("setup_future_usage", "off_session")
	}

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
	for k, v := range req.Metadata {
		params.Set("metadata["+k+"]", fmt.Sprintf("%v", v))
	}
	if k := strings.TrimSpace(req.IdempotencyKey); k != "" {
		params.Set("metadata["+orderKey+"]", fitKey(k))
	}

	var pi paymentIntent
	if err := p.postKey(ctx, "/payment_intents", params, req.IdempotencyKey, &pi); err != nil {
		return &processor.PaymentResult{
			Success:      false,
			ErrorMessage: err.Error(),
			Error:        err,
		}, err
	}
	return p.settled(&pi, off)
}

// orderKey is the metadata name a PaymentIntent carries the order's
// idempotency key under, which is how a later request proves the intent it
// names is its own.
const orderKey = "hanzo_key"

// settled is what a PaymentIntent's status means for the charge that made or
// read it.
func (p *Provider) settled(pi *paymentIntent, off bool) (*processor.PaymentResult, error) {
	switch pi.Status {
	case "succeeded":
		return &processor.PaymentResult{
			Success:       true,
			TransactionID: pi.ID,
			ProcessorRef:  pi.LatestCharge,
			Status:        pi.Status,
		}, nil
	case "requires_action":
		// A PaymentIntent that needs the buyer (3-D Secure) has moved no money.
		// With the buyer present the page finishes it; off session nobody can, so
		// it is a decline the payer answers by updating the card.
		res := &processor.PaymentResult{Success: false, TransactionID: pi.ID, ProcessorRef: pi.ID, Status: pi.Status}
		if off {
			err := processor.NewPaymentError(processor.Stripe, "authentication_required",
				"this card needs its bank's authentication, which a charge made without you cannot complete; update the card", nil)
			res.ErrorMessage, res.Error = err.Error(), err
			return res, err
		}
		err := &processor.ActionRequired{Processor: processor.Stripe, Intent: pi.ID, ClientSecret: pi.ClientSecret}
		res.ErrorMessage, res.Error = err.Error(), err
		return res, err
	}
	err := processor.NewPaymentError(processor.Stripe, "payment_incomplete",
		"the payment did not complete (status "+pi.Status+"); nothing was charged", nil)
	return &processor.PaymentResult{
		Success: false, TransactionID: pi.ID, ProcessorRef: pi.ID, Status: pi.Status,
		ErrorMessage: err.Error(), Error: err,
	}, err
}

// resume reads the PaymentIntent a buyer authenticated and answers it as the
// charge req describes, refusing one that is not req's own.
func (p *Provider) resume(ctx context.Context, req processor.PaymentRequest, id string) (*processor.PaymentResult, error) {
	if !strings.HasPrefix(id, "pi_") {
		err := processor.NewPaymentError(processor.Stripe, "invalid_intent", "paymentIntent must be a Stripe PaymentIntent id (pi_…)", nil)
		return &processor.PaymentResult{ErrorMessage: err.Error(), Error: err}, err
	}
	var pi paymentIntent
	if err := p.get(ctx, "/payment_intents/"+url.PathEscape(id), nil, &pi); err != nil {
		return &processor.PaymentResult{ErrorMessage: err.Error(), Error: err}, err
	}
	key := strings.TrimSpace(req.IdempotencyKey)
	own := key != "" && fmt.Sprint(pi.Metadata[orderKey]) == fitKey(key) &&
		pi.Amount == int64(req.Amount) &&
		strings.EqualFold(pi.Currency, string(req.Currency)) &&
		(req.CustomerID == "" || pi.Customer == req.CustomerID) &&
		(req.Token == "" || pi.PaymentMethod == req.Token)
	if !own {
		err := processor.NewPaymentError(processor.Stripe, "intent_mismatch",
			"the payment named is not this order's; nothing was charged", nil)
		return &processor.PaymentResult{TransactionID: pi.ID, ErrorMessage: err.Error(), Error: err}, err
	}
	return p.settled(&pi, false)
}

func optString(opts map[string]interface{}, key string) string {
	s, _ := opts[key].(string)
	return strings.TrimSpace(s)
}

func optBool(opts map[string]interface{}, key string) bool {
	b, _ := opts[key].(bool)
	return b
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
		Metadata:     pi.Metadata,
	}, nil
}

// ValidateWebhook verifies a Stripe webhook signature and parses the event.
//
// The signature is Stripe's scheme v1: HMAC-SHA256 under the endpoint's
// signing secret over "<t>.<raw body>", with t within webhookTolerance of now in
// either direction. The header may carry several v1 signatures while a secret
// is rolled; any one of them verifying is enough.
//
// The event keeps Stripe's own type (charge.refunded, charge.dispute.created,
// ...). What it means for the books is decided by the billing receiver, which
// also reads the event's livemode and created time, carried in Data as
// stripe_livemode and stripe_created beside the object's own fields.
func (p *Provider) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*processor.WebhookEvent, error) {
	if p.webhookSecret == "" {
		return nil, processor.ErrWebhookValidationFailed
	}
	timestamp, sigs := parseSignatures(signature)
	if timestamp == "" || len(sigs) == 0 {
		return nil, processor.ErrWebhookValidationFailed
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil, processor.ErrWebhookValidationFailed
	}
	if d := time.Since(time.Unix(ts, 0)); d > webhookTolerance || d < -webhookTolerance {
		return nil, processor.ErrWebhookValidationFailed
	}
	mac := hmac.New(sha256.New, []byte(p.webhookSecret))
	mac.Write([]byte(timestamp + "." + string(payload)))
	want := []byte(hex.EncodeToString(mac.Sum(nil)))
	ok := false
	for _, sig := range sigs {
		if hmac.Equal([]byte(sig), want) {
			ok = true
		}
	}
	if !ok {
		return nil, processor.ErrWebhookValidationFailed
	}

	var evt stripeEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return nil, fmt.Errorf("failed to parse webhook event: %w", err)
	}
	data := evt.Data.Object
	if data == nil {
		data = map[string]interface{}{}
	}
	data["stripe_livemode"] = evt.Livemode
	data["stripe_created"] = evt.Created
	return &processor.WebhookEvent{
		ID:        evt.ID,
		Type:      evt.Type,
		Processor: processor.Stripe,
		Data:      data,
		Timestamp: ts,
	}, nil
}

// webhookTolerance is how far a delivery's signed time may be from now:
// Stripe's own libraries default to five minutes.
const webhookTolerance = 5 * time.Minute

// ReceivesWebhooks reports whether this provider holds the endpoint signing
// secret its deliveries are verified with.
func (p *Provider) ReceivesWebhooks() bool { return p.webhookSecret != "" }

// parseSignatures reads a Stripe-Signature header: the t= timestamp and every
// v1= signature.
func parseSignatures(header string) (string, []string) {
	var t string
	var v1 []string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			t = v
		case "v1":
			v1 = append(v1, v)
		}
	}
	return t, v1
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

	// A customer made for a reference is made once: retried under the same
	// reference within Stripe's idempotency window it is the same customer.
	// The email is part of the key: Stripe refuses a key reused with other
	// parameters, and a second customer for a changed address is harmless where a
	// refused create would stop the checkout.
	key := ""
	if ref, _ := metadata["reference"].(string); ref != "" {
		key = "customer:" + ref + ":" + strings.ToLower(strings.TrimSpace(email))
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
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
			return processor.NewPaymentError(
				processor.Stripe,
				apiErr.Error.Code,
				apiErr.Error.Message,
				nil,
			)
		}
		return fmt.Errorf("stripe API error (HTTP %d): %s", resp.StatusCode, string(body))
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
	PaymentMethod string                 `json:"payment_method"`
	ClientSecret  string                 `json:"client_secret"`
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
	Livemode bool   `json:"livemode"`
	Created  int64  `json:"created"`
	Data     struct {
		Object map[string]interface{} `json:"object"`
	} `json:"data"`
}

type stripeAPIError struct {
	Error struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
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

// Compile-time interface checks.
var (
	_ processor.PaymentProcessor      = (*Provider)(nil)
	_ processor.SubscriptionProcessor = (*Provider)(nil)
	_ processor.CustomerProcessor     = (*Provider)(nil)
	_ processor.WebhookReceiver       = (*Provider)(nil)
)

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

var _ processor.Reach = (*Provider)(nil)
