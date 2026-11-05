// Package kraken implements the Kraken exchange CryptoProcessor for Hanzo Commerce.
//
// Why Kraken for crypto payments:
//   - Calling POST /0/private/DepositAddresses with new=true mints a fresh,
//     single-use deposit address from our Kraken exchange account for each
//     crypto payment (1 address per payment).
//   - Deposits into Kraken addresses are swept by Kraken into exchange
//     liquidity pools behind the scenes, so external observers cannot track a
//     single merchant treasury wallet or correlate payments across customers.
package kraken

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/money"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

const (
	defaultBaseURL = "https://api.kraken.com"
	defaultTimeout = 15 * time.Second
)

var supportedChains = []string{
	"ethereum",
	"base",
	"arbitrum",
	"optimism",
	"polygon",
	"avalanche",
	"solana",
	"bitcoin",
	"ton",
	"xrpl",
}

var supportedCurrencies = []currency.Type{
	currency.BTC,
	currency.XBT,
	currency.ETH,
	"sol",
	"usdc",
	"usdt",
	"rlusd",
	"ton",
	"xrp",
	"avax",
	"matic",
}

// Config holds Kraken API credentials and options.
type Config struct {
	APIKey    string
	APISecret string
	APIURL    string
	OTP       string
	Timeout   time.Duration
}

// Processor implements processor.PaymentProcessor and processor.CryptoProcessor
// backed by the Kraken Exchange Funding REST API.
type Processor struct {
	*processor.BaseProcessor
	apiKey     string
	apiSecret  []byte
	baseURL    string
	otp        string
	httpClient *http.Client

	nonceMu   sync.Mutex
	lastNonce int64

	methodMu    sync.RWMutex
	methodCache map[string][]DepositMethod
}

// DepositMethod represents a single entry returned by Kraken's
// /0/private/DepositMethods endpoint.
type DepositMethod struct {
	Method     string      `json:"method"`
	Limit      interface{} `json:"limit"`
	Fee        string      `json:"fee,omitempty"`
	GenAddress bool        `json:"gen-address"`
	Minimum    string      `json:"minimum,omitempty"`
}

// DepositAddress represents a single entry returned by Kraken's
// /0/private/DepositAddresses endpoint.
type DepositAddress struct {
	Address  string      `json:"address"`
	ExpireTm interface{} `json:"expiretm"`
	New      bool        `json:"new"`
	Tag      string      `json:"tag,omitempty"`
	Memo     string      `json:"memo,omitempty"`
}

// DepositRecord represents a single entry returned by Kraken's
// /0/private/DepositStatus endpoint.
type DepositRecord struct {
	Method  string `json:"method"`
	AClass  string `json:"aclass"`
	Asset   string `json:"asset"`
	RefID   string `json:"refid"`
	TxID    string `json:"txid"`
	Info    string `json:"info"`
	Amount  string `json:"amount"`
	Fee     string `json:"fee"`
	Time    int64  `json:"time"`
	Status  string `json:"status"`
	StatusP string `json:"status-prop,omitempty"`
}

type krakenEnvelope struct {
	Error  []string        `json:"error"`
	Result json.RawMessage `json:"result"`
}

func init() {
	p := NewProcessor(Config{
		APIKey:    strings.TrimSpace(os.Getenv("KRAKEN_API_KEY")),
		APISecret: strings.TrimSpace(os.Getenv("KRAKEN_API_SECRET")),
		APIURL:    strings.TrimSpace(os.Getenv("KRAKEN_API_URL")),
		OTP:       strings.TrimSpace(os.Getenv("KRAKEN_OTP")),
	})
	processor.Register(p)
}

// NewProcessor creates a Kraken processor instance from the given Config.
func NewProcessor(cfg Config) *Processor {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.APIURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	p := &Processor{
		BaseProcessor: processor.NewBaseProcessor(processor.Kraken, supportedCurrencies),
		apiKey:        strings.TrimSpace(cfg.APIKey),
		baseURL:       baseURL,
		otp:           strings.TrimSpace(cfg.OTP),
		httpClient:    &http.Client{Timeout: timeout},
		methodCache:   make(map[string][]DepositMethod),
	}
	p.configureCredentials(cfg.APIKey, cfg.APISecret)
	return p
}

// Configure reloads credentials at runtime.
func (p *Processor) Configure(cfg Config) {
	if strings.TrimSpace(cfg.APIURL) != "" {
		p.baseURL = strings.TrimRight(strings.TrimSpace(cfg.APIURL), "/")
	}
	if strings.TrimSpace(cfg.OTP) != "" {
		p.otp = strings.TrimSpace(cfg.OTP)
	}
	p.configureCredentials(cfg.APIKey, cfg.APISecret)
}

func (p *Processor) configureCredentials(apiKey, apiSecret string) {
	apiKey = strings.TrimSpace(apiKey)
	apiSecret = strings.TrimSpace(apiSecret)
	if apiKey == "" || apiSecret == "" {
		p.SetConfigured(false)
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(apiSecret)
	if err != nil || len(decoded) == 0 {
		p.SetConfigured(false)
		return
	}
	p.apiKey = apiKey
	p.apiSecret = decoded
	p.SetConfigured(true)
}

// IsAvailable reports whether Kraken API credentials are configured (checking
// environment variables dynamically if credentials were injected after init).
func (p *Processor) IsAvailable(ctx context.Context) bool {
	if p.BaseProcessor.IsAvailable(ctx) {
		return true
	}
	key := strings.TrimSpace(os.Getenv("KRAKEN_API_KEY"))
	secret := strings.TrimSpace(os.Getenv("KRAKEN_API_SECRET"))
	if key != "" && secret != "" {
		if urlEnv := strings.TrimSpace(os.Getenv("KRAKEN_API_URL")); urlEnv != "" {
			p.baseURL = strings.TrimRight(urlEnv, "/")
		}
		if otpEnv := strings.TrimSpace(os.Getenv("KRAKEN_OTP")); otpEnv != "" {
			p.otp = otpEnv
		}
		p.configureCredentials(key, secret)
	}
	return p.BaseProcessor.IsAvailable(ctx)
}

// SupportedChains returns the blockchain networks supported via Kraken.
func (p *Processor) SupportedChains() []string {
	out := make([]string, len(supportedChains))
	copy(out, supportedChains)
	return out
}

// GenerateAddress requests a brand-new deposit address (new=true) from Kraken
// for the given customer and chain (with token read from processor.TokenFromContext).
//
// Each crypto payment gets its own fresh Kraken deposit address so external
// observers cannot link payments or track a single merchant wallet.
func (p *Processor) GenerateAddress(ctx context.Context, customerID string, chain string) (processor.Wallet, error) {
	if !p.IsAvailable(ctx) {
		return processor.Wallet{}, processor.NewPaymentError(processor.Kraken, "NOT_CONFIGURED",
			"kraken processor is not configured (missing KRAKEN_API_KEY or KRAKEN_API_SECRET)", nil)
	}

	chain = strings.ToLower(strings.TrimSpace(chain))
	if chain == "" {
		chain = "ethereum"
	}
	if !isChainSupported(chain) {
		return processor.Wallet{}, processor.NewPaymentError(processor.Kraken, "UNSUPPORTED_CHAIN",
			fmt.Sprintf("kraken: unsupported chain %q", chain), nil)
	}

	token := strings.ToLower(strings.TrimSpace(processor.TokenFromContext(ctx)))
	asset := krakenAsset(token, chain)

	method, err := p.resolveDepositMethod(ctx, chain, token, asset)
	if err != nil {
		return processor.Wallet{}, err
	}

	// Always request a brand-new deposit address (new=true) for each payment so
	// every crypto payment receives a unique, single-use address. If Kraken
	// reports EFunding:Too many addresses (when 5 consecutive unused addresses
	// have already been minted on that network without a deposit landing yet),
	// fall back to new=false and select one of the unused addresses.
	addrs, err := p.fetchDepositAddresses(ctx, asset, method, true)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "too many addresses") {
		addrs, err = p.fetchDepositAddresses(ctx, asset, method, false)
	}
	if err != nil {
		return processor.Wallet{}, err
	}

	chosen, ok := pickDepositAddress(addrs)
	if !ok {
		return processor.Wallet{}, processor.NewPaymentError(processor.Kraken, "NO_ADDRESS",
			fmt.Sprintf("kraken: DepositAddresses returned no usable address for asset=%s method=%q", asset, method), nil)
	}

	tag := strings.TrimSpace(chosen.Tag)
	if tag == "" {
		tag = strings.TrimSpace(chosen.Memo)
	}

	return processor.Wallet{
		Address: chosen.Address,
		ID:      fmt.Sprintf("kraken:%s:%s:%s", chain, strings.ToLower(asset), chosen.Address),
		Tag:     tag,
	}, nil
}

// ListDeposits queries Kraken's /0/private/DepositStatus for recent deposits of
// (chain, token).
func (p *Processor) ListDeposits(ctx context.Context, chain, token string) ([]DepositRecord, error) {
	if !p.IsAvailable(ctx) {
		return nil, processor.NewPaymentError(processor.Kraken, "NOT_CONFIGURED", "kraken processor not configured", nil)
	}
	chain = strings.ToLower(strings.TrimSpace(chain))
	token = strings.ToLower(strings.TrimSpace(token))
	asset := krakenAsset(token, chain)

	form := url.Values{}
	form.Set("asset", asset)
	if chain != "" {
		if method, err := p.resolveDepositMethod(ctx, chain, token, asset); err == nil && method != "" {
			form.Set("method", method)
		}
	}

	var records []DepositRecord
	if err := p.privatePost(ctx, "/0/private/DepositStatus", form, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// GetBalance checks deposits recorded by Kraken for a specific deposit address.
func (p *Processor) GetBalance(ctx context.Context, address string, chain string) (*processor.Balance, error) {
	if !p.IsAvailable(ctx) {
		return nil, processor.NewPaymentError(processor.Kraken, "NOT_CONFIGURED", "kraken processor not configured", nil)
	}
	token := processor.TokenFromContext(ctx)
	records, err := p.ListDeposits(ctx, chain, token)
	if err != nil {
		return nil, err
	}

	var available, pending currency.Cents
	for _, r := range records {
		if !strings.EqualFold(strings.TrimSpace(r.Info), strings.TrimSpace(address)) {
			continue
		}
		cents := parseDecimalToCents(r.Amount)
		switch strings.ToLower(strings.TrimSpace(r.Status)) {
		case "success", "settled":
			available += cents
		case "pending", "initial":
			pending += cents
		}
	}

	return &processor.Balance{
		Available: available,
		Pending:   pending,
		Currency:  currency.USD,
	}, nil
}

// EstimateFee returns 0 for inbound Kraken crypto deposits (payer pays network gas).
func (p *Processor) EstimateFee(ctx context.Context, req processor.PaymentRequest) (currency.Cents, error) {
	return 0, nil
}

// Charge verifies or initiates a crypto deposit payment via Kraken.
func (p *Processor) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if !p.IsAvailable(ctx) {
		return nil, processor.NewPaymentError(processor.Kraken, "NOT_CONFIGURED", "kraken processor not configured", nil)
	}
	chain := req.Chain
	if chain == "" {
		chain = "ethereum"
	}
	wallet, err := p.GenerateAddress(ctx, req.CustomerID, chain)
	if err != nil {
		return &processor.PaymentResult{
			Success:      false,
			ErrorMessage: err.Error(),
			Error:        err,
			Status:       "failed",
		}, err
	}
	return &processor.PaymentResult{
		Success:       true,
		TransactionID: wallet.ID,
		ProcessorRef:  wallet.Address,
		Status:        "pending",
		Metadata: map[string]interface{}{
			"depositAddress": wallet.Address,
			"walletId":       wallet.ID,
			"tag":            wallet.Tag,
			"chain":          chain,
		},
	}, nil
}

// Authorize is not supported for push crypto deposits.
func (p *Processor) Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	return nil, processor.NewPaymentError(processor.Kraken, "NOT_SUPPORTED", "authorize not supported for kraken crypto deposits", nil)
}

// Capture is not supported for push crypto deposits.
func (p *Processor) Capture(ctx context.Context, transactionID string, amount money.Amount) (*processor.PaymentResult, error) {
	return nil, processor.NewPaymentError(processor.Kraken, "NOT_SUPPORTED", "capture not supported for kraken crypto deposits", nil)
}

// Refund is not supported automatically via Deposit APIs.
func (p *Processor) Refund(ctx context.Context, req processor.RefundRequest) (*processor.RefundResult, error) {
	return nil, processor.NewPaymentError(processor.Kraken, "NOT_SUPPORTED", "automated refund not supported for kraken deposit rail", nil)
}

// GetTransaction looks up a deposit by txid or refid in Kraken's DepositStatus.
func (p *Processor) GetTransaction(ctx context.Context, transactionID string) (*processor.Transaction, error) {
	if !p.IsAvailable(ctx) {
		return nil, processor.NewPaymentError(processor.Kraken, "NOT_CONFIGURED", "kraken processor not configured", nil)
	}
	records, err := p.ListDeposits(ctx, "", "usdc")
	if err != nil {
		return nil, err
	}
	for _, r := range records {
		if r.TxID == transactionID || r.RefID == transactionID {
			return &processor.Transaction{
				ID:           r.RefID,
				ProcessorRef: r.TxID,
				Type:         "deposit",
				Amount:       parseDecimalToCents(r.Amount),
				Currency:     currency.Type(strings.ToLower(r.Asset)),
				Status:       strings.ToLower(r.Status),
				CreatedAt:    r.Time,
				UpdatedAt:    r.Time,
			}, nil
		}
	}
	return nil, processor.NewPaymentError(processor.Kraken, "NOT_FOUND", fmt.Sprintf("kraken deposit %q not found", transactionID), nil)
}

// ValidateWebhook validates Kraken webhook callbacks if configured.
func (p *Processor) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*processor.WebhookEvent, error) {
	return nil, processor.ErrWebhookValidationFailed
}

func (p *Processor) fetchDepositAddresses(ctx context.Context, asset, method string, requestNew bool) ([]DepositAddress, error) {
	form := url.Values{}
	form.Set("asset", asset)
	form.Set("method", method)
	if requestNew {
		form.Set("new", "true")
	}

	var addrs []DepositAddress
	if err := p.privatePost(ctx, "/0/private/DepositAddresses", form, &addrs); err != nil {
		return nil, err
	}
	return addrs, nil
}

func (p *Processor) resolveDepositMethod(ctx context.Context, chain, token, asset string) (string, error) {
	// 1. Check explicit environment overrides first:
	//    KRAKEN_DEPOSIT_METHOD_<CHAIN>_<TOKEN> or KRAKEN_DEPOSIT_METHOD_<CHAIN>
	chainUpper := strings.ToUpper(chain)
	tokenUpper := strings.ToUpper(token)
	if tokenUpper != "" {
		if m := strings.TrimSpace(os.Getenv("KRAKEN_DEPOSIT_METHOD_" + chainUpper + "_" + tokenUpper)); m != "" {
			return m, nil
		}
	}
	if m := strings.TrimSpace(os.Getenv("KRAKEN_DEPOSIT_METHOD_" + chainUpper)); m != "" {
		return m, nil
	}

	// 2. Query Kraken's /0/private/DepositMethods for the asset (cached per asset).
	methods, err := p.depositMethodsForAsset(ctx, asset)
	if err != nil {
		return "", err
	}
	if len(methods) == 0 {
		return "", processor.NewPaymentError(processor.Kraken, "NO_DEPOSIT_METHODS",
			fmt.Sprintf("kraken: no deposit methods available for asset %s", asset), nil)
	}

	if matched, ok := matchMethodForChain(methods, chain); ok {
		return matched, nil
	}

	return "", processor.NewPaymentError(processor.Kraken, "UNSUPPORTED_METHOD",
		fmt.Sprintf("kraken: no deposit method on chain %q for asset %s", chain, asset), nil)
}

func (p *Processor) depositMethodsForAsset(ctx context.Context, asset string) ([]DepositMethod, error) {
	p.methodMu.RLock()
	cached, ok := p.methodCache[asset]
	p.methodMu.RUnlock()
	if ok && len(cached) > 0 {
		return cached, nil
	}

	form := url.Values{}
	form.Set("asset", asset)

	var methods []DepositMethod
	if err := p.privatePost(ctx, "/0/private/DepositMethods", form, &methods); err != nil {
		return nil, err
	}

	if len(methods) > 0 {
		p.methodMu.Lock()
		p.methodCache[asset] = methods
		p.methodMu.Unlock()
	}
	return methods, nil
}

// matchMethodForChain selects the Kraken deposit method corresponding to chain.
//
// Kraken supports chain-specific methods (e.g. "Arbitrum One", "Polygon",
// "Solana", "Base") as well as Unified EVM Deposit Methods where all EVM
// networks share an "Ethereum" / "ERC-20" / unified method on the account.
func matchMethodForChain(methods []DepositMethod, chain string) (string, bool) {
	candidates := chainMethodKeywords(chain)
	// First pass: prefer canonical native asset methods over bridged/variant
	// prefixes (such as "USDC.e", "USDT0", "USDCx").
	for _, allowVariant := range []bool{false, true} {
		for _, kw := range candidates {
			for _, m := range methods {
				name := strings.ToLower(strings.TrimSpace(m.Method))
				if name == "" {
					continue
				}
				// On Bitcoin, never accidentally match "Bitcoin Lightning" unless lightning was asked.
				if chain == "bitcoin" && strings.Contains(name, "lightning") {
					continue
				}
				if !allowVariant && (strings.Contains(name, ".e") || strings.HasPrefix(name, "usdt0") || strings.HasPrefix(name, "usdcx")) {
					continue
				}
				if strings.Contains(name, kw) {
					return m.Method, true
				}
			}
		}
	}
	return "", false
}

func chainMethodKeywords(chain string) []string {
	switch chain {
	case "ethereum":
		return []string{"ethereum (erc20)", "ethereum (erc-20)", "erc-20", "erc20", "ethereum", "unified", "ether"}
	case "base":
		return []string{"base", "unified", "ethereum (erc20)", "ethereum (erc-20)", "erc-20", "erc20", "ethereum"}
	case "arbitrum":
		return []string{"arbitrum one", "arbitrum", "unified", "ethereum (erc20)", "ethereum (erc-20)", "erc-20", "erc20", "ethereum"}
	case "optimism":
		return []string{"optimism", "op mainnet", "unified", "ethereum (erc20)", "ethereum (erc-20)", "erc-20", "erc20", "ethereum"}
	case "polygon":
		return []string{"polygon", "matic", "unified", "ethereum (erc20)", "ethereum (erc-20)", "erc-20", "erc20", "ethereum"}
	case "avalanche":
		return []string{"avalanche", "avax", "c-chain", "unified", "ethereum (erc20)", "ethereum (erc-20)", "erc-20", "erc20", "ethereum"}
	case "solana":
		return []string{"solana", "spl"}
	case "bitcoin":
		return []string{"bitcoin"}
	case "ton":
		return []string{"ton"}
	case "xrpl":
		return []string{"ripple", "xrp", "xrpl"}
	default:
		return []string{chain}
	}
}

func krakenAsset(token, chain string) string {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "usdc":
		return "USDC"
	case "usdt":
		return "USDT"
	case "rlusd":
		return "RLUSD"
	case "dai":
		return "DAI"
	case "pyusd":
		return "PYUSD"
	case "btc", "xbt":
		return "XBT"
	case "eth":
		return "XETH"
	case "sol":
		return "SOL"
	case "ton":
		return "TON"
	case "xrp":
		return "XXRP"
	case "matic", "pol":
		return "POL"
	case "avax":
		return "AVAX"
	case "":
		switch chain {
		case "bitcoin":
			return "XBT"
		case "solana":
			return "SOL"
		case "ton":
			return "TON"
		case "xrpl":
			return "XXRP"
		default:
			return "USDC"
		}
	default:
		return strings.ToUpper(strings.TrimSpace(token))
	}
}

func isChainSupported(chain string) bool {
	for _, c := range supportedChains {
		if c == chain {
			return true
		}
	}
	return false
}

func pickDepositAddress(addrs []DepositAddress) (DepositAddress, bool) {
	// Prefer a newly generated address (New: true) that has a non-empty Address.
	for _, a := range addrs {
		if strings.TrimSpace(a.Address) != "" && a.New {
			return a, true
		}
	}
	for _, a := range addrs {
		if strings.TrimSpace(a.Address) != "" {
			return a, true
		}
	}
	return DepositAddress{}, false
}

func (p *Processor) nextNonce() string {
	p.nonceMu.Lock()
	defer p.nonceMu.Unlock()
	now := time.Now().UnixNano()
	if now <= p.lastNonce {
		now = p.lastNonce + 1
	}
	p.lastNonce = now
	return strconv.FormatInt(now, 10)
}

// signKrakenRequest computes the Kraken API-Sign header value:
//
//	base64(HMAC-SHA512(uriPath + SHA256(nonce + postData), base64Decode(secret)))
func signKrakenRequest(uriPath, nonce, postData string, secret []byte) string {
	sha := sha256.Sum256([]byte(nonce + postData))
	mac := hmac.New(sha512.New, secret)
	mac.Write([]byte(uriPath))
	mac.Write(sha[:])
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (p *Processor) privatePost(ctx context.Context, uriPath string, form url.Values, out interface{}) error {
	if form == nil {
		form = url.Values{}
	}
	nonce := p.nextNonce()
	form.Set("nonce", nonce)
	if p.otp != "" && form.Get("otp") == "" {
		form.Set("otp", p.otp)
	}

	postData := form.Encode()
	sig := signKrakenRequest(uriPath, nonce, postData, p.apiSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+uriPath, strings.NewReader(postData))
	if err != nil {
		return processor.NewPaymentError(processor.Kraken, "REQUEST_ERROR", "failed to build kraken request", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("API-Key", p.apiKey)
	req.Header.Set("API-Sign", sig)
	req.Header.Set("User-Agent", "HanzoCommerce/1.0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return processor.NewPaymentError(processor.Kraken, "NETWORK_ERROR", "kraken API request failed", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return processor.NewPaymentError(processor.Kraken, "READ_ERROR", "failed to read kraken response", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return processor.NewPaymentError(processor.Kraken, fmt.Sprintf("HTTP_%d", resp.StatusCode),
			fmt.Sprintf("kraken HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))), nil)
	}

	var env krakenEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return processor.NewPaymentError(processor.Kraken, "PARSE_ERROR", "invalid JSON from kraken", err)
	}
	if len(env.Error) > 0 {
		joined := strings.Join(env.Error, "; ")
		return processor.NewPaymentError(processor.Kraken, "API_ERROR",
			fmt.Sprintf("kraken error: %s", joined), errors.New(joined))
	}

	if out != nil && len(env.Result) > 0 {
		if err := json.Unmarshal(env.Result, out); err != nil {
			return processor.NewPaymentError(processor.Kraken, "RESULT_PARSE_ERROR",
				"failed to decode kraken result payload", err)
		}
	}
	return nil
}

func parseDecimalToCents(s string) currency.Cents {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	cents, err := currency.USD.Parse(s)
	if err != nil {
		return 0
	}
	return cents
}

var (
	_ processor.PaymentProcessor = (*Processor)(nil)
	_ processor.CryptoProcessor  = (*Processor)(nil)
)
