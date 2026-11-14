package paypal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/money"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

const (
	liveBaseURL    = "https://api-m.paypal.com"
	sandboxBaseURL = "https://api-m.sandbox.paypal.com"
	tokenPath      = "/v1/oauth2/token"
	ordersPath     = "/v2/checkout/orders"
	authCapture    = "/v2/payments/authorizations"
	capturesPath   = "/v2/payments/captures"
	webhookVerify  = "/v1/notifications/verify-webhook-signature"
)

// Config holds PayPal API credentials.
type Config struct {
	ClientID            string
	ClientSecret        string
	WebhookID           string
	Sandbox             bool
	Org                 string
	SandboxClientID     string
	SandboxClientSecret string
	SandboxWebhookID    string
}

// Provider implements processor.PaymentProcessor and processor.WebhookReceiver
// for PayPal REST API v2.
type Provider struct {
	*processor.BaseProcessor

	mu                 sync.RWMutex
	config             Config
	httpClient         *http.Client
	accessToken        string
	tokenExpiry        time.Time
	sandboxAccessToken string
	sandboxTokenExpiry time.Time
}

func init() {
	p := &Provider{
		BaseProcessor: processor.NewBaseProcessor(processor.PayPal, supportedCurrencies()),
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
	processor.Register(p)
}

// Reconfigure reads the deployment's PayPal accounts from the routing policy.
func (p *Provider) Reconfigure() { p.Configure(configFromAccounts()) }

func configFromAccounts() Config {
	sandbox := SandboxEnv()
	creds := Deployment(sandbox)
	sbCreds := Deployment(true)
	return Config{
		ClientID:            creds.ClientID,
		ClientSecret:        creds.ClientSecret,
		WebhookID:           creds.WebhookID,
		Sandbox:             sandbox,
		SandboxClientID:     sbCreds.ClientID,
		SandboxClientSecret: sbCreds.ClientSecret,
		SandboxWebhookID:    sbCreds.WebhookID,
	}
}

// NewProvider creates a configured PayPal provider instance.
func NewProvider(cfg Config) *Provider {
	p := &Provider{
		BaseProcessor: processor.NewBaseProcessor(processor.PayPal, supportedCurrencies()),
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
	p.Configure(cfg)
	return p
}

// Configure sets PayPal credentials and marks the processor as available.
func (p *Provider) Configure(cfg Config) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = cfg
	p.accessToken = ""
	p.tokenExpiry = time.Time{}
	p.sandboxAccessToken = ""
	p.sandboxTokenExpiry = time.Time{}
	p.SetConfigured(cfg.ClientID != "" && cfg.ClientSecret != "")
}

// Type returns the processor type.
func (p *Provider) Type() processor.ProcessorType {
	return processor.PayPal
}

// IsAvailable reports whether the processor is configured.
func (p *Provider) IsAvailable(_ context.Context) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.ClientID != "" && p.config.ClientSecret != ""
}

// ReceivesWebhooks reports whether this account holds the credentials and
// webhook subscription ID needed to verify inbound PayPal webhook deliveries.
func (p *Provider) ReceivesWebhooks() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	primary := p.config.ClientID != "" && p.config.ClientSecret != "" && p.config.WebhookID != ""
	sandbox := p.config.SandboxClientID != "" && p.config.SandboxClientSecret != "" && p.config.SandboxWebhookID != ""
	return primary || sandbox
}

// SupportedCurrencies returns currencies supported by PayPal.
func (p *Provider) SupportedCurrencies() []currency.Type {
	return supportedCurrencies()
}

func supportedCurrencies() []currency.Type {
	return []currency.Type{
		currency.USD, currency.EUR, currency.GBP, currency.CAD,
		currency.AUD, currency.JPY, currency.CHF, currency.NZD,
		currency.SGD, currency.HKD, currency.NOK, currency.SEK,
		currency.DKK, currency.PLN, currency.BRL, currency.MXN,
		currency.CZK, currency.HUF, currency.ILS, currency.MYR,
		currency.PHP, currency.TWD, currency.THB, currency.RUB,
		currency.INR,
	}
}

// ---------------------------------------------------------------------------
// Order creation & PaymentProcessor interface
// ---------------------------------------------------------------------------

// CreateOrder creates a server-side PayPal REST v2 order (intent "CAPTURE" or
// "AUTHORIZE") so the PayPal JS SDK can approve a server-authoritative amount.
func (p *Provider) CreateOrder(ctx context.Context, req processor.PaymentRequest, intent string) (string, string, error) {
	if err := p.checkAvailable(); err != nil {
		return "", "", err
	}
	if err := processor.ValidateRequest(req); err != nil {
		return "", "", err
	}
	intent = strings.ToUpper(strings.TrimSpace(intent))
	if intent != "AUTHORIZE" {
		intent = "CAPTURE"
	}
	created, err := p.createOrder(ctx, req, intent)
	if err != nil {
		return "", "", err
	}
	return created.ID, created.Status, nil
}

// Charge creates a PayPal order with CAPTURE intent and captures it immediately.
// If req.Token is set, it is treated as an existing approved order ID to capture.
func (p *Provider) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := p.checkAvailable(); err != nil {
		return nil, err
	}
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}

	orderID := req.Token
	if orderID == "" {
		// Create a new order with CAPTURE intent.
		created, err := p.createOrder(ctx, req, "CAPTURE")
		if err != nil {
			return nil, err
		}
		orderID = created.ID
	}

	// Capture the order.
	capture, err := p.captureOrderWithKey(ctx, orderID, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}

	card, wallet := paidWith(capture)
	result := &processor.PaymentResult{
		Success:       capture.Status == "COMPLETED",
		TransactionID: capture.ID,
		ProcessorRef:  captureIDFromOrder(capture),
		Status:        capture.Status,
		Card:          card,
		Wallet:        wallet,
		Metadata:      map[string]interface{}{"paypal_order_id": capture.ID},
	}
	if !result.Success {
		result.ErrorMessage = fmt.Sprintf("paypal: order capture status %s", capture.Status)
		result.Error = processor.ErrPaymentFailed
		return result, nil
	}
	if req.Amount > 0 {
		if capCents, capCur := capturedOrderCents(capture); capCents > 0 {
			if capCents != int64(req.Amount) {
				result.Success = false
				result.Error = processor.ErrPaymentFailed
				result.ErrorMessage = fmt.Sprintf("paypal: captured amount %d cents does not match requested amount %d cents", capCents, req.Amount)
				return result, nil
			}
			if capCur != "" && !strings.EqualFold(capCur, req.Currency.Code()) {
				result.Success = false
				result.Error = processor.ErrPaymentFailed
				result.ErrorMessage = fmt.Sprintf("paypal: captured currency %s does not match requested currency %s", capCur, req.Currency.Code())
				return result, nil
			}
		}
	}
	return result, nil
}

// Authorize creates a PayPal order with AUTHORIZE intent and authorizes it.
// If req.Token is set, it is treated as an existing approved order ID to authorize.
func (p *Provider) Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := p.checkAvailable(); err != nil {
		return nil, err
	}
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}

	orderID := req.Token
	if orderID == "" {
		created, err := p.createOrder(ctx, req, "AUTHORIZE")
		if err != nil {
			return nil, err
		}
		orderID = created.ID
	}

	auth, err := p.authorizeOrderWithKey(ctx, orderID, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}

	authID := authorizationIDFromOrder(auth)
	card, wallet := paidWith(auth)
	result := &processor.PaymentResult{
		Success:       auth.Status == "COMPLETED",
		TransactionID: auth.ID,
		ProcessorRef:  authID,
		Status:        auth.Status,
		Card:          card,
		Wallet:        wallet,
		Metadata: map[string]interface{}{
			"paypal_order_id":         auth.ID,
			"paypal_authorization_id": authID,
		},
	}
	if !result.Success {
		result.ErrorMessage = fmt.Sprintf("paypal: order authorize status %s", auth.Status)
		result.Error = processor.ErrPaymentFailed
		return result, nil
	}
	if req.Amount > 0 {
		if authCents, authCur := authorizedOrderCents(auth); authCents > 0 {
			if authCents != int64(req.Amount) {
				result.Success = false
				result.Error = processor.ErrPaymentFailed
				result.ErrorMessage = fmt.Sprintf("paypal: authorized amount %d cents does not match requested amount %d cents", authCents, req.Amount)
				return result, nil
			}
			if authCur != "" && !strings.EqualFold(authCur, req.Currency.Code()) {
				result.Success = false
				result.Error = processor.ErrPaymentFailed
				result.ErrorMessage = fmt.Sprintf("paypal: authorized currency %s does not match requested currency %s", authCur, req.Currency.Code())
				return result, nil
			}
		}
	}
	return result, nil
}

// CancelAuthorization voids an uncaptured PayPal authorization.
func (p *Provider) CancelAuthorization(ctx context.Context, authorizationID string) error {
	if err := p.checkAvailable(); err != nil {
		return err
	}
	authorizationID = strings.TrimSpace(authorizationID)
	if authorizationID == "" {
		return processor.ErrInvalidPaymentRequest
	}
	path := fmt.Sprintf("%s/%s/void", authCapture, authorizationID)
	_, err := p.doRequest(ctx, http.MethodPost, path, nil)
	return err
}

// Capture captures a previously authorized payment.
// transactionID is the PayPal authorization ID returned in Authorize metadata.
func (p *Provider) Capture(ctx context.Context, transactionID string, amount money.Amount) (*processor.PaymentResult, error) {
	if err := p.checkAvailable(); err != nil {
		return nil, err
	}
	if transactionID == "" {
		return nil, processor.ErrInvalidPaymentRequest
	}

	body := map[string]interface{}{
		"amount": map[string]string{
			"value":         amount.MajorString(),
			"currency_code": amount.Currency().Code,
		},
	}
	// If amount is 0, capture the full authorized amount (omit body).
	var payload []byte
	var err error
	if amount.Sign() > 0 {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, p.payErr("MARSHAL_ERROR", "failed to marshal capture body", err)
		}
	}

	path := fmt.Sprintf("%s/%s/capture", authCapture, transactionID)
	resp, err := p.doRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return nil, err
	}

	var capture paypalCapture
	if err := json.Unmarshal(resp, &capture); err != nil {
		return nil, p.payErr("PARSE_ERROR", "failed to parse capture response", err)
	}

	result := &processor.PaymentResult{
		Success:       capture.Status == "COMPLETED",
		TransactionID: capture.ID,
		ProcessorRef:  capture.ID,
		Status:        capture.Status,
		Metadata:      map[string]interface{}{"paypal_capture_id": capture.ID},
	}
	if !result.Success {
		result.ErrorMessage = fmt.Sprintf("paypal: capture status %s", capture.Status)
		result.Error = processor.ErrPaymentFailed
		return result, nil
	}
	if amount.Sign() > 0 && capture.Amount.Value != "" {
		cur := currency.Type(strings.ToLower(capture.Amount.CurrencyCode))
		expCur := currency.Type(strings.ToLower(amount.Currency().Code))
		expCents, _ := expCur.Parse(amount.MajorString())
		if cents, err := cur.Parse(capture.Amount.Value); err == nil && expCents > 0 && cents != expCents {
			result.Success = false
			result.Error = processor.ErrPaymentFailed
			result.ErrorMessage = fmt.Sprintf("paypal: captured amount %d cents does not match expected amount %d cents", cents, expCents)
			return result, nil
		}
	}
	return result, nil
}

// Refund refunds a captured payment.
// req.TransactionID is the PayPal capture ID.
func (p *Provider) Refund(ctx context.Context, req processor.RefundRequest) (*processor.RefundResult, error) {
	if err := p.checkAvailable(); err != nil {
		return nil, err
	}
	if req.TransactionID == "" {
		return nil, processor.ErrInvalidPaymentRequest
	}

	body := make(map[string]interface{})
	if req.Amount.Sign() > 0 {
		body["amount"] = map[string]string{
			"value":         req.Amount.MajorString(),
			"currency_code": req.Amount.Currency().Code,
		}
	}
	if req.Reason != "" {
		body["note_to_payer"] = req.Reason
	}

	var payload []byte
	var err error
	if len(body) > 0 {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, p.payErr("MARSHAL_ERROR", "failed to marshal refund body", err)
		}
	}

	path := fmt.Sprintf("%s/%s/refund", capturesPath, req.TransactionID)
	resp, _, err := p.doRequestDetailed(ctx, http.MethodPost, path, payload, req.IdempotencyKey)
	if err != nil {
		return &processor.RefundResult{
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
		}, err
	}

	var refund paypalRefund
	if err := json.Unmarshal(resp, &refund); err != nil {
		return nil, p.payErr("PARSE_ERROR", "failed to parse refund response", err)
	}

	success := refund.Status == "COMPLETED"
	result := &processor.RefundResult{
		Success:      success,
		RefundID:     refund.ID,
		ProcessorRef: refund.ID,
	}
	if !success {
		result.ErrorMessage = fmt.Sprintf("paypal: refund status %s", refund.Status)
		result.Error = processor.ErrRefundFailed
	}
	return result, nil
}

// GetTransaction retrieves order details by order ID, falling back to a payment
// authorization lookup when txID is a PayPal authorization ID (e.g. during
// walletSale settlement recovery).
func (p *Provider) GetTransaction(ctx context.Context, txID string) (*processor.Transaction, error) {
	if err := p.checkAvailable(); err != nil {
		return nil, err
	}
	if txID == "" {
		return nil, processor.ErrTransactionNotFound
	}

	path := fmt.Sprintf("%s/%s", ordersPath, txID)
	resp, status, err := p.doRequestDetailed(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		if status == http.StatusNotFound {
			if tx, authErr := p.getAuthorizationTransaction(ctx, txID); authErr == nil {
				return tx, nil
			}
		}
		return nil, err
	}

	var order paypalOrder
	if err := json.Unmarshal(resp, &order); err != nil {
		return nil, p.payErr("PARSE_ERROR", "failed to parse order response", err)
	}

	tx := &processor.Transaction{
		ID:           order.ID,
		ProcessorRef: order.ID,
		Status:       order.Status,
		Metadata: map[string]interface{}{
			"intent":    order.Intent,
			"payer":     order.Payer,
			"links":     order.Links,
			"raw_order": order,
		},
	}

	// Extract amount from first purchase unit.
	if len(order.PurchaseUnits) > 0 {
		pu := order.PurchaseUnits[0]
		// The order carries its own currency, and that currency's decimal
		// convention decides the scale — a zero-decimal currency (JPY) must not
		// gain two. commerce's currency table is the authority on that.
		tx.Currency = currency.Type(strings.ToLower(pu.Amount.CurrencyCode))
		amount, err := tx.Currency.Parse(pu.Amount.Value)
		if err != nil {
			return nil, fmt.Errorf("paypal order %s: %w", order.ID, err)
		}
		tx.Amount = amount
	}

	// Map PayPal status to transaction type.
	switch order.Intent {
	case "CAPTURE":
		tx.Type = "charge"
	case "AUTHORIZE":
		tx.Type = "authorize"
	default:
		tx.Type = "charge"
	}

	if order.CreateTime != "" {
		if t, err := time.Parse(time.RFC3339, order.CreateTime); err == nil {
			tx.CreatedAt = t.Unix()
		}
	}
	if order.UpdateTime != "" {
		if t, err := time.Parse(time.RFC3339, order.UpdateTime); err == nil {
			tx.UpdatedAt = t.Unix()
		}
	}

	return tx, nil
}

func (p *Provider) getAuthorizationTransaction(ctx context.Context, authID string) (*processor.Transaction, error) {
	path := fmt.Sprintf("%s/%s", authCapture, authID)
	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var auth paypalAuthorizationDetail
	if err := json.Unmarshal(resp, &auth); err != nil {
		return nil, p.payErr("PARSE_ERROR", "failed to parse authorization response", err)
	}

	mappedStatus := strings.ToUpper(strings.TrimSpace(auth.Status))
	switch mappedStatus {
	case "CAPTURED":
		mappedStatus = "COMPLETED"
	case "CREATED", "PENDING", "PARTIALLY_CAPTURED":
		mappedStatus = "APPROVED"
	case "VOIDED", "DENIED", "EXPIRED":
		mappedStatus = "CANCELED"
	}

	tx := &processor.Transaction{
		ID:           auth.ID,
		ProcessorRef: auth.ID,
		Type:         "authorize",
		Status:       mappedStatus,
		Metadata: map[string]interface{}{
			"paypal_authorization_status": auth.Status,
		},
	}
	if auth.Amount.CurrencyCode != "" && auth.Amount.Value != "" {
		tx.Currency = currency.Type(strings.ToLower(auth.Amount.CurrencyCode))
		if amt, err := tx.Currency.Parse(auth.Amount.Value); err == nil {
			tx.Amount = amt
		}
	}
	if auth.CreateTime != "" {
		if t, err := time.Parse(time.RFC3339, auth.CreateTime); err == nil {
			tx.CreatedAt = t.Unix()
		}
	}
	if auth.UpdateTime != "" {
		if t, err := time.Parse(time.RFC3339, auth.UpdateTime); err == nil {
			tx.UpdatedAt = t.Unix()
		}
	}
	return tx, nil
}

// ValidateWebhook verifies a PayPal webhook notification.
// signature should contain the JSON-encoded transmission headers:
//
//	{"transmission_id":"...", "transmission_time":"...", "cert_url":"...",
//	 "auth_algo":"...", "transmission_sig":"..."}
//
// payload is the raw webhook event body.
func (p *Provider) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*processor.WebhookEvent, error) {
	// Parse the transmission headers from the signature string first so a dual-configured
	// global singleton can select sandbox vs live verification based on cert_url.
	var headers struct {
		TransmissionID   string `json:"transmission_id"`
		TransmissionTime string `json:"transmission_time"`
		CertURL          string `json:"cert_url"`
		AuthAlgo         string `json:"auth_algo"`
		TransmissionSig  string `json:"transmission_sig"`
	}

	p.mu.RLock()
	cfg := p.config
	p.mu.RUnlock()

	useSandboxAlt := false
	webhookID := cfg.WebhookID

	if signature != "" {
		if err := json.Unmarshal([]byte(signature), &headers); err != nil {
			if cfg.ClientID == "" || cfg.ClientSecret == "" {
				return nil, processor.NewPaymentError(processor.PayPal, "NOT_CONFIGURED", "paypal processor not configured", nil)
			}
			if webhookID == "" {
				return nil, processor.ErrWebhookValidationFailed
			}
			return nil, p.payErr("WEBHOOK_PARSE", "failed to parse webhook signature headers", err)
		}
	}

	isSandboxCert := isPayPalSandboxCertURL(headers.CertURL)
	if !cfg.Sandbox && isSandboxCert && cfg.SandboxClientID != "" && cfg.SandboxClientSecret != "" && cfg.SandboxWebhookID != "" {
		useSandboxAlt = true
		webhookID = cfg.SandboxWebhookID
	} else {
		if cfg.ClientID == "" || cfg.ClientSecret == "" {
			return nil, processor.NewPaymentError(processor.PayPal, "NOT_CONFIGURED", "paypal processor not configured", nil)
		}
	}

	if webhookID == "" {
		return nil, processor.ErrWebhookValidationFailed
	}

	if headers.CertURL != "" {
		u, uerr := url.Parse(headers.CertURL)
		if uerr != nil || !strings.EqualFold(u.Scheme, "https") {
			return nil, processor.ErrWebhookValidationFailed
		}
	}

	// Parse the event body to include in verification request.
	var eventBody json.RawMessage
	if err := json.Unmarshal(payload, &eventBody); err != nil {
		return nil, p.payErr("WEBHOOK_PARSE", "failed to parse webhook payload", err)
	}

	verifyReq := map[string]interface{}{
		"auth_algo":         headers.AuthAlgo,
		"cert_url":          headers.CertURL,
		"transmission_id":   headers.TransmissionID,
		"transmission_sig":  headers.TransmissionSig,
		"transmission_time": headers.TransmissionTime,
		"webhook_id":        webhookID,
		"webhook_event":     eventBody,
	}

	reqBody, err := json.Marshal(verifyReq)
	if err != nil {
		return nil, p.payErr("MARSHAL_ERROR", "failed to marshal verify request", err)
	}

	var resp []byte
	if useSandboxAlt {
		resp, err = p.doSandboxWebhookVerify(ctx, reqBody)
	} else {
		resp, err = p.doRequest(ctx, http.MethodPost, webhookVerify, reqBody)
	}
	if err != nil {
		return nil, err
	}

	var verifyResp struct {
		VerificationStatus string `json:"verification_status"`
	}
	if err := json.Unmarshal(resp, &verifyResp); err != nil {
		return nil, p.payErr("PARSE_ERROR", "failed to parse verification response", err)
	}

	if verifyResp.VerificationStatus != "SUCCESS" {
		return nil, processor.ErrWebhookValidationFailed
	}

	// Parse the original event.
	var event struct {
		ID           string                 `json:"id"`
		EventType    string                 `json:"event_type"`
		Resource     map[string]interface{} `json:"resource"`
		CreateTime   string                 `json:"create_time"`
		ResourceType string                 `json:"resource_type"`
		Summary      string                 `json:"summary"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, p.payErr("PARSE_ERROR", "failed to parse webhook event", err)
	}

	var ts int64
	if event.CreateTime != "" {
		if t, err := time.Parse(time.RFC3339, event.CreateTime); err == nil {
			ts = t.Unix()
		}
	}

	data := map[string]interface{}{
		"resource":      event.Resource,
		"resource_type": event.ResourceType,
		"summary":       event.Summary,
	}
	if useSandboxAlt || (cfg.Sandbox && isSandboxCert) {
		data["paypal_sandbox"] = true
	}

	return &processor.WebhookEvent{
		ID:        event.ID,
		Type:      event.EventType,
		Processor: processor.PayPal,
		Org:       cfg.Org,
		Data:      data,
		Timestamp: ts,
	}, nil
}

func isPayPalSandboxCertURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "api.sandbox.paypal.com" || h == "api-m.sandbox.paypal.com" || strings.HasSuffix(h, ".sandbox.paypal.com")
}

// ---------------------------------------------------------------------------
// OAuth2 token management
// ---------------------------------------------------------------------------

// getAccessToken returns a valid access token, refreshing if expired.
func (p *Provider) getAccessToken(ctx context.Context) (string, error) {
	p.mu.RLock()
	token := p.accessToken
	expiry := p.tokenExpiry
	p.mu.RUnlock()

	// Return cached token if still valid with 60s buffer.
	if token != "" && time.Now().Before(expiry.Add(-60*time.Second)) {
		return token, nil
	}

	return p.refreshToken(ctx)
}

// refreshToken fetches a new OAuth2 access token from PayPal.
func (p *Provider) refreshToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock.
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry.Add(-60*time.Second)) {
		return p.accessToken, nil
	}

	token, expiresIn, err := p.fetchOAuthToken(ctx, p.baseURL(), p.config.ClientID, p.config.ClientSecret)
	if err != nil {
		return "", err
	}

	p.accessToken = token
	p.tokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)

	return p.accessToken, nil
}

func (p *Provider) getSandboxAccessToken(ctx context.Context) (string, error) {
	p.mu.RLock()
	token := p.sandboxAccessToken
	expiry := p.sandboxTokenExpiry
	p.mu.RUnlock()

	if token != "" && time.Now().Before(expiry.Add(-60*time.Second)) {
		return token, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sandboxAccessToken != "" && time.Now().Before(p.sandboxTokenExpiry.Add(-60*time.Second)) {
		return p.sandboxAccessToken, nil
	}

	tok, expiresIn, err := p.fetchOAuthToken(ctx, sandboxBaseURL, p.config.SandboxClientID, p.config.SandboxClientSecret)
	if err != nil {
		return "", err
	}
	p.sandboxAccessToken = tok
	p.sandboxTokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return p.sandboxAccessToken, nil
}

func (p *Provider) fetchOAuthToken(ctx context.Context, base, clientID, clientSecret string) (string, int, error) {
	body := strings.NewReader("grant_type=client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+tokenPath, body)
	if err != nil {
		return "", 0, p.payErr("TOKEN_REQUEST", "failed to create token request", err)
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", 0, p.payErr("TOKEN_NETWORK", "failed to fetch access token", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, p.payErr("TOKEN_READ", "failed to read token response", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, p.payErr("TOKEN_ERROR",
			fmt.Sprintf("paypal token endpoint returned %d: %s", resp.StatusCode, string(respBody)), nil)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", 0, p.payErr("TOKEN_PARSE", "failed to parse token response", err)
	}

	if tokenResp.AccessToken == "" {
		return "", 0, p.payErr("TOKEN_EMPTY", "paypal returned empty access token", nil)
	}

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

func (p *Provider) doSandboxWebhookVerify(ctx context.Context, reqBody []byte) ([]byte, error) {
	token, err := p.getSandboxAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sandboxBaseURL+webhookVerify, bytes.NewReader(reqBody))
	if err != nil {
		return nil, p.payErr("REQUEST_BUILD", "failed to build request", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, p.payErr("NETWORK_ERROR", "paypal api request failed", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, p.payErr("READ_ERROR", "failed to read paypal response", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return respBody, nil
	}
	return nil, p.payErr("API_ERROR",
		fmt.Sprintf("paypal POST %s returned %d: %s", webhookVerify, resp.StatusCode, string(respBody)), nil)
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func (p *Provider) baseURL() string {
	if p.config.Sandbox {
		return sandboxBaseURL
	}
	return liveBaseURL
}

// doRequest executes an authenticated request against the PayPal API.
func (p *Provider) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	resp, _, err := p.doRequestDetailed(ctx, method, path, body, "")
	return resp, err
}

func (p *Provider) doRequestDetailed(ctx context.Context, method, path string, body []byte, idempotencyKey string) ([]byte, int, error) {
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, 0, err
	}

	base := p.baseURL()
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, base+path, reqBody)
	if err != nil {
		return nil, 0, p.payErr("REQUEST_BUILD", "failed to build request", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if idempotencyKey = strings.TrimSpace(idempotencyKey); idempotencyKey != "" && method == http.MethodPost {
		req.Header.Set("PayPal-Request-Id", fitKey(idempotencyKey))
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, 0, p.payErr("NETWORK_ERROR", "paypal api request failed", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, p.payErr("READ_ERROR", "failed to read paypal response", err)
	}

	// PayPal returns 200, 201, or 204 for success.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return respBody, resp.StatusCode, nil
	}

	// Parse PayPal error response for detail.
	var apiErr paypalAPIError
	if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Name != "" {
		return nil, resp.StatusCode, p.payErr(apiErr.Name,
			fmt.Sprintf("paypal %s %s: %s - %s", method, path, apiErr.Name, apiErr.Message), nil)
	}

	return nil, resp.StatusCode, p.payErr("API_ERROR",
		fmt.Sprintf("paypal %s %s returned %d: %s", method, path, resp.StatusCode, string(respBody)), nil)
}

// said marks a definitive 4xx refusal from PayPal (other than 408 or an
// in-flight idempotency key collision) as a processor.Answer so callers such as
// walletSale know PayPal reached a "no" decision and holds no authorization.
func said(status int, err error) error {
	if err == nil {
		return nil
	}
	if status >= 400 && status < 500 && status != http.StatusRequestTimeout {
		msg := strings.ToUpper(err.Error())
		if !strings.Contains(msg, "IDEMPOTENCY_REQUEST_IN_PROGRESS") && !strings.Contains(msg, "DUPLICATE_REQUEST_ID") {
			return &processor.Answer{Err: err}
		}
	}
	return err
}

// ---------------------------------------------------------------------------
// PayPal order operations
// ---------------------------------------------------------------------------

// createOrder creates a new PayPal order.
func (p *Provider) createOrder(ctx context.Context, req processor.PaymentRequest, intent string) (*paypalOrder, error) {
	cur := req.Currency
	orderReq := map[string]interface{}{
		"intent": intent,
		"purchase_units": []map[string]interface{}{
			{
				"amount": map[string]string{
					"currency_code": cur.Code(),
					"value":         cur.ToStringNoSymbol(req.Amount),
				},
			},
		},
		"application_context": map[string]string{
			"shipping_preference": "NO_SHIPPING",
			"user_action":         "PAY_NOW",
		},
	}

	// Add description if present.
	if req.Description != "" {
		pu := orderReq["purchase_units"].([]map[string]interface{})
		pu[0]["description"] = req.Description
	}

	// Add order reference.
	if req.OrderID != "" {
		pu := orderReq["purchase_units"].([]map[string]interface{})
		pu[0]["reference_id"] = req.OrderID
		pu[0]["invoice_id"] = req.OrderID
	}

	// Add custom metadata.
	if req.CustomerID != "" {
		pu := orderReq["purchase_units"].([]map[string]interface{})
		pu[0]["custom_id"] = req.CustomerID
	}

	body, err := json.Marshal(orderReq)
	if err != nil {
		return nil, p.payErr("MARSHAL_ERROR", "failed to marshal order request", err)
	}

	resp, status, err := p.doRequestDetailed(ctx, http.MethodPost, ordersPath, body, req.IdempotencyKey)
	if err != nil {
		return nil, said(status, err)
	}

	var order paypalOrder
	if err := json.Unmarshal(resp, &order); err != nil {
		return nil, p.payErr("PARSE_ERROR", "failed to parse create order response", err)
	}

	return &order, nil
}

// captureOrder captures an approved order.
func (p *Provider) captureOrder(ctx context.Context, orderID string) (*paypalOrder, error) {
	return p.captureOrderWithKey(ctx, orderID, "")
}

func (p *Provider) captureOrderWithKey(ctx context.Context, orderID, idemKey string) (*paypalOrder, error) {
	path := fmt.Sprintf("%s/%s/capture", ordersPath, orderID)

	// PayPal expects an empty JSON body or no body for capture.
	resp, status, err := p.doRequestDetailed(ctx, http.MethodPost, path, []byte("{}"), idemKey)
	if err != nil {
		return nil, said(status, err)
	}

	var order paypalOrder
	if err := json.Unmarshal(resp, &order); err != nil {
		return nil, p.payErr("PARSE_ERROR", "failed to parse capture order response", err)
	}

	return &order, nil
}

// authorizeOrder authorizes an approved order.
func (p *Provider) authorizeOrder(ctx context.Context, orderID string) (*paypalOrder, error) {
	return p.authorizeOrderWithKey(ctx, orderID, "")
}

func (p *Provider) authorizeOrderWithKey(ctx context.Context, orderID, idemKey string) (*paypalOrder, error) {
	path := fmt.Sprintf("%s/%s/authorize", ordersPath, orderID)

	resp, status, err := p.doRequestDetailed(ctx, http.MethodPost, path, []byte("{}"), idemKey)
	if err != nil {
		return nil, said(status, err)
	}

	var order paypalOrder
	if err := json.Unmarshal(resp, &order); err != nil {
		return nil, p.payErr("PARSE_ERROR", "failed to parse authorize order response", err)
	}

	return &order, nil
}

// ---------------------------------------------------------------------------
// PayPal API response types (internal)
// ---------------------------------------------------------------------------

type paypalOrder struct {
	ID            string               `json:"id"`
	Intent        string               `json:"intent"`
	Status        string               `json:"status"`
	PurchaseUnits []paypalPurchaseUnit `json:"purchase_units"`
	PaymentSource *paypalPaymentSource `json:"payment_source,omitempty"`
	Payer         interface{}          `json:"payer"`
	Links         interface{}          `json:"links"`
	CreateTime    string               `json:"create_time"`
	UpdateTime    string               `json:"update_time"`
}

type paypalPaymentSource struct {
	Card   *paypalCardSource   `json:"card,omitempty"`
	PayPal *paypalWalletSource `json:"paypal,omitempty"`
}

type paypalCardSource struct {
	LastDigits string `json:"last_digits"`
	Brand      string `json:"brand"`
	Expiry     string `json:"expiry"` // "YYYY-MM"
}

type paypalWalletSource struct {
	AccountID    string `json:"account_id"`
	EmailAddress string `json:"email_address"`
}

type paypalPurchaseUnit struct {
	ReferenceID string          `json:"reference_id"`
	Amount      paypalAmount    `json:"amount"`
	Payments    *paypalPayments `json:"payments,omitempty"`
	Description string          `json:"description"`
	CustomID    string          `json:"custom_id"`
	InvoiceID   string          `json:"invoice_id"`
}

type paypalAmount struct {
	CurrencyCode string `json:"currency_code"`
	Value        string `json:"value"`
}

type paypalPayments struct {
	Captures       []paypalCapture       `json:"captures,omitempty"`
	Authorizations []paypalAuthorization `json:"authorizations,omitempty"`
	Refunds        []paypalRefund        `json:"refunds,omitempty"`
}

type paypalCapture struct {
	ID     string       `json:"id"`
	Status string       `json:"status"`
	Amount paypalAmount `json:"amount"`
}

type paypalAuthorization struct {
	ID     string       `json:"id"`
	Status string       `json:"status"`
	Amount paypalAmount `json:"amount"`
}

type paypalAuthorizationDetail struct {
	ID         string       `json:"id"`
	Status     string       `json:"status"`
	Amount     paypalAmount `json:"amount"`
	CreateTime string       `json:"create_time"`
	UpdateTime string       `json:"update_time"`
}

type paypalRefund struct {
	ID     string       `json:"id"`
	Status string       `json:"status"`
	Amount paypalAmount `json:"amount"`
}

type paypalAPIError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	DebugID string `json:"debug_id"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// paidWith extracts card/account details from a captured or authorized PayPal
// order so one-use-per-card/payer coupon rules (walletCard) can key on it.
func paidWith(order *paypalOrder) (*processor.Card, string) {
	if order == nil {
		return nil, ""
	}
	if ps := order.PaymentSource; ps != nil {
		if c := ps.Card; c != nil && (c.Brand != "" || c.LastDigits != "") {
			var expYear, expMonth int
			if yearStr, monthStr, ok := strings.Cut(strings.TrimSpace(c.Expiry), "-"); ok {
				expYear, _ = strconv.Atoi(yearStr)
				expMonth, _ = strconv.Atoi(monthStr)
			}
			return &processor.Card{
				Brand:    c.Brand,
				Last4:    c.LastDigits,
				ExpYear:  expYear,
				ExpMonth: expMonth,
			}, "PAYPAL"
		}
		if w := ps.PayPal; w != nil && strings.TrimSpace(w.AccountID) != "" {
			return &processor.Card{
				Fingerprint: "paypal:" + strings.TrimSpace(w.AccountID),
			}, "PAYPAL"
		}
	}
	if payerMap, ok := order.Payer.(map[string]interface{}); ok {
		if payerID, _ := payerMap["payer_id"].(string); strings.TrimSpace(payerID) != "" {
			return &processor.Card{
				Fingerprint: "paypal:" + strings.TrimSpace(payerID),
			}, "PAYPAL"
		}
	}
	return nil, "PAYPAL"
}

// captureIDFromOrder extracts the first capture ID from order purchase units.
func captureIDFromOrder(order *paypalOrder) string {
	for _, pu := range order.PurchaseUnits {
		if pu.Payments != nil && len(pu.Payments.Captures) > 0 {
			return pu.Payments.Captures[0].ID
		}
	}
	return order.ID
}

// authorizationIDFromOrder extracts the first authorization ID from order purchase units.
func authorizationIDFromOrder(order *paypalOrder) string {
	for _, pu := range order.PurchaseUnits {
		if pu.Payments != nil && len(pu.Payments.Authorizations) > 0 {
			return pu.Payments.Authorizations[0].ID
		}
	}
	return order.ID
}

// capturedOrderCents extracts the first capture amount in minor units and currency code.
func capturedOrderCents(order *paypalOrder) (int64, string) {
	if order == nil {
		return 0, ""
	}
	for _, pu := range order.PurchaseUnits {
		if pu.Payments != nil && len(pu.Payments.Captures) > 0 {
			c := pu.Payments.Captures[0]
			if c.Amount.Value != "" {
				cur := currency.Type(strings.ToLower(c.Amount.CurrencyCode))
				if cents, err := cur.Parse(c.Amount.Value); err == nil {
					return int64(cents), strings.ToUpper(c.Amount.CurrencyCode)
				}
			}
		}
		if pu.Amount.Value != "" {
			cur := currency.Type(strings.ToLower(pu.Amount.CurrencyCode))
			if cents, err := cur.Parse(pu.Amount.Value); err == nil {
				return int64(cents), strings.ToUpper(pu.Amount.CurrencyCode)
			}
		}
	}
	return 0, ""
}

// authorizedOrderCents extracts the first authorization amount in minor units and currency code.
func authorizedOrderCents(order *paypalOrder) (int64, string) {
	if order == nil {
		return 0, ""
	}
	for _, pu := range order.PurchaseUnits {
		if pu.Payments != nil && len(pu.Payments.Authorizations) > 0 {
			a := pu.Payments.Authorizations[0]
			if a.Amount.Value != "" {
				cur := currency.Type(strings.ToLower(a.Amount.CurrencyCode))
				if cents, err := cur.Parse(a.Amount.Value); err == nil {
					return int64(cents), strings.ToUpper(a.Amount.CurrencyCode)
				}
			}
		}
		if pu.Amount.Value != "" {
			cur := currency.Type(strings.ToLower(pu.Amount.CurrencyCode))
			if cents, err := cur.Parse(pu.Amount.Value); err == nil {
				return int64(cents), strings.ToUpper(pu.Amount.CurrencyCode)
			}
		}
	}
	return 0, ""
}

func (p *Provider) checkAvailable() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.config.ClientID == "" || p.config.ClientSecret == "" {
		return processor.NewPaymentError(processor.PayPal, "NOT_CONFIGURED", "paypal processor not configured", nil)
	}
	return nil
}

func (p *Provider) payErr(code, msg string, err error) *processor.PaymentError {
	return processor.NewPaymentError(processor.PayPal, code, msg, err)
}

// UnwrapPaymentError extracts the underlying *processor.PaymentError even when
// wrapped in *processor.Answer.
func UnwrapPaymentError(err error) (*processor.PaymentError, bool) {
	var pe *processor.PaymentError
	if errors.As(err, &pe) {
		return pe, true
	}
	return nil, false
}

// Compile-time interface checks.
var (
	_ processor.PaymentProcessor = (*Provider)(nil)
	_ processor.WebhookReceiver  = (*Provider)(nil)
)
