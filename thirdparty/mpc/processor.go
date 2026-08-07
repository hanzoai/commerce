package mpc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hanzoai/money"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// MPCProcessor implements the processor.CryptoProcessor interface
// using Hanzo KMS (control plane) + MPC Signer (signing backend).
type MPCProcessor struct {
	*processor.BaseProcessor
	kmsEndpoint   string
	mpcEndpoint   string
	webhookSecret string

	// transport carries keygen and health, and which wire it is came from
	// configuration at construction. Nothing below this field ever asks: that
	// is what makes the wire a deployment fact rather than a call site's
	// business.
	transport Transport

	// http carries the transactional calls, which have no wire but this one —
	// mpcd's ZAP surface registers no opcode for creating, approving,
	// refunding or querying a transaction. When no ZAP address is configured
	// this is also the Transport above, so the HTTP plumbing exists once.
	http *httpTransport
}

// Config holds MPC processor configuration.
type Config struct {
	KMSEndpoint string // Hanzo KMS API endpoint
	MPCEndpoint string // Hanzo MPC Signer endpoint
	APIKey      string // API key for authentication
	// WebhookSecret is the per-subscription secret the MPC service signs
	// deliveries with. It is a DIFFERENT value from APIKey: APIKey authenticates
	// our outbound calls, WebhookSecret authenticates the service's inbound
	// notifications, and the two are provisioned independently.
	WebhookSecret string

	// ZAPAddress is where mpcd's ZAP surface listens, and naming it is what
	// selects that wire for keygen and health. Empty — the default everywhere
	// no one has said otherwise — keeps the HTTP wire, so a standalone
	// commerce needs no configuration to keep behaving as it always has.
	//
	// The value also chooses the medium, because luxfi/zap derives the network
	// from the address: a host:port is TCP, and a filesystem path (or an "@"
	// abstract name) is a unix socket. One setting, rather than a flag and an
	// address that can disagree with it.
	//
	// There is no port to put here yet. mpcd defines an MPC-API ZAP surface but
	// never starts it — pkg/api StartZAP has no callers — so nothing listens
	// for these opcodes on any port or socket in the fleet today.
	ZAPAddress string
}

// DefaultConfig reads the rail's configuration from the environment.
//
// It is the ONE place a variable becomes configuration — init registers the
// processor from it — so a variable that is not read here is a variable that
// does nothing. That was not a hypothetical: this function previously read a
// set of names (MPC_API_URL, KMS_API_URL, defaulting to localhost) that nothing
// called it with, while registration built its own Config from a DIFFERENT set,
// so the two could not agree and the dead one silently claimed the good name.
func DefaultConfig() Config {
	kmsEndpoint := strings.TrimSpace(os.Getenv("MPC_KMS_ENDPOINT"))
	if kmsEndpoint == "" {
		kmsEndpoint = "https://kms.hanzo.ai"
	}

	return Config{
		KMSEndpoint: kmsEndpoint,
		MPCEndpoint: strings.TrimSpace(os.Getenv("MPC_ENDPOINT")),
		APIKey:      strings.TrimSpace(os.Getenv("MPC_API_KEY")),

		// The secret the MPC service signs its deliveries with, separate from
		// the API key we authenticate outbound calls with. Unset means inbound
		// webhooks are refused, so turning the rail on means provisioning both.
		WebhookSecret: strings.TrimSpace(os.Getenv("MPC_WEBHOOK_SECRET")),

		// Where mpcd's ZAP surface listens, and naming it is what moves keygen
		// and health onto that wire. Unset — every deployment until one says
		// otherwise — leaves the rail on HTTP exactly as before.
		ZAPAddress: strings.TrimSpace(os.Getenv("MPC_ZAP_ADDR")),
	}
}

// NewProcessor creates a new MPC processor.
func NewProcessor(cfg Config) *MPCProcessor {
	h := &httpTransport{
		endpoint: strings.TrimRight(cfg.MPCEndpoint, "/"),
		apiKey:   cfg.APIKey,
		client:   &http.Client{Timeout: keygenTimeout},
	}
	mp := &MPCProcessor{
		BaseProcessor: processor.NewBaseProcessor(processor.MPC, MPCSupportedCurrencies()),
		kmsEndpoint:   strings.TrimRight(cfg.KMSEndpoint, "/"),
		mpcEndpoint:   h.endpoint,
		webhookSecret: cfg.WebhookSecret,
		transport:     newTransport(cfg, h),
		http:          h,
	}

	if mp.configured() {
		mp.SetConfigured(true)
	}

	return mp
}

// configured reports whether the rail has the endpoints it needs to exist at
// all. This is the pure half of availability — it answers "is this rail turned
// on" with no I/O, which is what a request-triggered path needs: IsAvailable
// also probes the remote, and an unauthenticated caller must never be able to
// make us dial out just by posting.
//
// A ZAP address does not substitute for the HTTP endpoint and is deliberately
// not consulted here. Selecting ZAP moves keygen and health onto that wire; the
// transactional calls have no ZAP opcode to move to, so a rail without an HTTP
// endpoint is still a rail that cannot finish a payment.
func (mp *MPCProcessor) configured() bool {
	return mp.kmsEndpoint != "" && mp.mpcEndpoint != ""
}

// MPCSupportedCurrencies returns cryptocurrencies supported by MPC.
func MPCSupportedCurrencies() []currency.Type {
	// SOL IS NOT HERE, and its absence is the honest state rather than an
	// oversight. A Solana address is an Ed25519 (EdDSA) key, and the custody
	// fleet only ever runs the secp256k1 ceremony: the mpc:generate consumer
	// dispatches handleKeyGenEventCGGMP21 and nothing else, so EDDSAPubKey comes
	// back empty and mpcd never emits sol_address (it is gated on a 32-byte
	// EdDSA key). GenerateAddress then finds nothing for chain "solana" and
	// answers NO_ADDRESS, which the rail renders as 503 "the custody service is
	// not accepting new addresses".
	//
	// Advertising it anyway is the worst of the options: the asset picker is
	// rendered FROM THIS LIST, so listing sol puts a button in front of a buyer
	// that walks them to a dead end after they have chosen an amount. A rail is
	// offered only when it can be finished.
	//
	// Restoring sol needs a FROST/Ed25519 ceremony beside the CGGMP21 one and
	// the two results combined per wallet — keygen_session.go already speaks of
	// "the combined result with both ECDSA and EdDSA keys", so the result shape
	// is ready and the consumer is what is missing. That is protocol work on a
	// live custody service, not a list edit.
	return []currency.Type{
		currency.BTC,
		currency.ETH,
		currency.Type("usdc"),
		currency.Type("usdt"),
		currency.Type("matic"),
		currency.Type("avax"),
		// The brand chains. Both are EVM, so a deposit address on either IS the
		// secp256k1 address the fleet already derives — GenerateAddress falls to
		// the EVM default for every chain that is not bitcoin or solana. That is
		// why these cost nothing to accept and why SOL, which needs a different
		// curve entirely, could not simply be listed beside them.
		currency.LUX,
		currency.ZOO,
		currency.Type("arb"),
		currency.Type("op"),
		currency.Type("base"),
	}
}

// SupportedChains returns blockchain networks supported by MPC.
func (mp *MPCProcessor) SupportedChains() []string {
	return []string{
		"bitcoin",
		"ethereum",
		"polygon",
		"arbitrum",
		"optimism",
		"base",
		"avalanche",
		// "solana" is omitted for the same reason sol is absent above: the
		// custody fleet derives no Ed25519 key, so a Solana deposit address
		// cannot be minted and the chain would 503 after the buyer picked it.
		"lux",
		"zoo",
		"bsc",
	}
}

// Type returns the processor type.
func (mp *MPCProcessor) Type() processor.ProcessorType {
	return processor.MPC
}

// --- MPC API types (matching lux/mpc/pkg/api) ---

type mpcCreateTxReq struct {
	WalletID  string `json:"wallet_id"`
	TxType    string `json:"tx_type"`
	Chain     string `json:"chain"`
	ToAddress string `json:"to_address"`
	Amount    string `json:"amount"`
	Token     string `json:"token,omitempty"`
}

type mpcTxResp struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	TxHash string `json:"txHash,omitempty"`
	Chain  string `json:"chain,omitempty"`
	Amount string `json:"amount,omitempty"`
}

type mpcBalanceResp struct {
	Address  string `json:"address"`
	Chain    string `json:"chain"`
	Balance  string `json:"balance"`
	Decimals int    `json:"decimals"`
}

// --- CryptoProcessor methods ---

// mpcKeygenResp is the live luxfi/mpc node's POST /keygen response: ONE
// threshold keygen yields the wallet's address on every chain class at once
// (secp256k1 → EVM + BTC, ed25519 → SOL).
type mpcKeygenResp struct {
	WalletID   string `json:"wallet_id"`
	ResultType string `json:"result_type"`
	EVMAddress string `json:"evm_address"`
	BTCAddress string `json:"btc_address"`
	SOLAddress string `json:"sol_address"`
	Error      string `json:"error"`
	ErrorCode  string `json:"error_code"`
}

// GenerateAddress runs a threshold keygen on the MPC signer fleet and returns
// the requested chain's address. The signer's reply is the same JSON on every
// wire:
//
//	{wallet_id, result_type, evm_address, btc_address, sol_address, ...}
//
// HOW that reply was fetched — JSON over HTTP to mpcd's REST surface, or a ZAP
// opcode over TCP or a unix socket — is settled once at construction and is
// deliberately not visible from here. Everything below the transport call is
// policy and is written exactly once, so the chain switch, the wallet-id
// capture and the refusal of an empty address are the same rules no matter
// which wire carried the bytes; the transports are tested against these very
// assertions to keep it that way.
//
// A fresh wallet per call is correct, never wasteful: callers that want address
// reuse hold on to the intent they recorded (the billing crypto-deposit rail
// does exactly that). Keygen needs ALL peers and can take tens of seconds — the
// caller's ctx bounds the wait, under a transport-wide ceiling.
func (mp *MPCProcessor) GenerateAddress(ctx context.Context, customerID string, chain string) (processor.Wallet, error) {
	// org_id scopes the wallet on the MPC side; the payer key is "<org>" or
	// "<org>/<user>", so the org is everything before the first slash.
	orgID := customerID
	if i := strings.IndexByte(orgID, '/'); i > 0 {
		orgID = orgID[:i]
	}

	raw, err := mp.transport.Keygen(ctx, orgID)
	if err != nil {
		return processor.Wallet{}, processor.NewPaymentError(processor.MPC, "KEYGEN_FAILED", "failed to generate MPC wallet", err)
	}
	var resp mpcKeygenResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return processor.Wallet{}, processor.NewPaymentError(processor.MPC, "KEYGEN_FAILED", "failed to decode MPC keygen result", err)
	}
	if resp.Error != "" {
		return processor.Wallet{}, processor.NewPaymentError(processor.MPC, "KEYGEN_FAILED", resp.Error, nil)
	}

	// resp.WalletID rides along with every address below. It was parsed here and
	// then dropped for want of somewhere to put it, which is how a rail came to
	// mint custody addresses it held no handle to.
	var addr string
	switch chain {
	case "bitcoin":
		addr = resp.BTCAddress
	case "solana":
		addr = resp.SOLAddress
	default:
		// EVM chains all share the secp256k1-derived Ethereum address.
		addr = resp.EVMAddress
	}
	if addr == "" {
		return processor.Wallet{}, processor.NewPaymentError(processor.MPC, "NO_ADDRESS", fmt.Sprintf("MPC keygen did not return address for chain %s", chain), nil)
	}
	return processor.Wallet{Address: addr, ID: resp.WalletID}, nil
}

// GetBalance retrieves the balance for an address on a given chain.
// Calls MPC service for balance lookup or chain RPC.
func (mp *MPCProcessor) GetBalance(ctx context.Context, address string, chain string) (*processor.Balance, error) {
	reqURL := fmt.Sprintf("%s/api/v1/wallets/%s/addresses", mp.mpcEndpoint, address)

	var addresses map[string]string
	err := mp.http.doJSON(ctx, http.MethodGet, reqURL, nil, &addresses)
	if err != nil {
		return &processor.Balance{
			Available: currency.Cents(0),
			Pending:   currency.Cents(0),
			Currency:  currency.Type(chain),
		}, processor.NewPaymentError(processor.MPC, "BALANCE_QUERY_FAILED", "failed to query balance", err)
	}

	return &processor.Balance{
		Available: currency.Cents(0),
		Pending:   currency.Cents(0),
		Currency:  currency.Type(chain),
	}, nil
}

// EstimateFee estimates transaction fees based on chain type.
func (mp *MPCProcessor) EstimateFee(ctx context.Context, req processor.PaymentRequest) (currency.Cents, error) {
	chain := req.Chain
	if chain == "" {
		chain = "ethereum"
	}

	// Chain-specific base fee estimates in USD cents.
	// These are conservative estimates; production should query chain RPCs.
	switch chain {
	case "bitcoin":
		return currency.Cents(500), nil // ~$5.00
	case "ethereum":
		return currency.Cents(300), nil // ~$3.00
	case "polygon", "arbitrum", "optimism", "base", "bsc":
		return currency.Cents(10), nil // ~$0.10
	case "avalanche", "lux":
		return currency.Cents(25), nil // ~$0.25
	case "solana":
		return currency.Cents(1), nil // ~$0.01
	default:
		return currency.Cents(100), nil // ~$1.00 fallback
	}
}

// Charge processes a crypto payment by creating and signing a transaction via MPC.
func (mp *MPCProcessor) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}

	chain := req.Chain
	if chain == "" {
		chain = chainForCurrency(req.Currency)
	}

	toAddress := req.Address
	if toAddress == "" {
		return nil, processor.NewPaymentError(processor.MPC, "NO_ADDRESS", "destination address required for crypto payment", nil)
	}

	// Create transaction via MPC API: POST /api/v1/transactions
	txReqURL := fmt.Sprintf("%s/api/v1/transactions", mp.mpcEndpoint)

	var txResp mpcTxResp
	err := mp.http.doJSON(ctx, http.MethodPost, txReqURL, &mpcCreateTxReq{
		WalletID:  req.CustomerID,
		TxType:    "transfer",
		Chain:     chain,
		ToAddress: toAddress,
		Amount:    fmt.Sprintf("%d", req.Amount),
		Token:     string(req.Currency),
	}, &txResp)
	if err != nil {
		return nil, processor.NewPaymentError(processor.MPC, "TX_CREATE_FAILED", "failed to create MPC transaction", err)
	}

	fee, _ := mp.EstimateFee(ctx, req)

	return &processor.PaymentResult{
		Success:       true,
		TransactionID: txResp.ID,
		ProcessorRef:  txResp.ID,
		Fee:           fee,
		Status:        txResp.Status,
		Metadata: map[string]interface{}{
			"chain":   chain,
			"address": toAddress,
			"txHash":  txResp.TxHash,
		},
	}, nil
}

// Authorize creates a pending transaction in the MPC policy engine for approval.
func (mp *MPCProcessor) Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}

	chain := req.Chain
	if chain == "" {
		chain = chainForCurrency(req.Currency)
	}

	// Create transaction that requires approval via MPC API
	txReqURL := fmt.Sprintf("%s/api/v1/transactions", mp.mpcEndpoint)

	var txResp mpcTxResp
	err := mp.http.doJSON(ctx, http.MethodPost, txReqURL, &mpcCreateTxReq{
		WalletID:  req.CustomerID,
		TxType:    "transfer",
		Chain:     chain,
		ToAddress: req.Address,
		Amount:    fmt.Sprintf("%d", req.Amount),
		Token:     string(req.Currency),
	}, &txResp)
	if err != nil {
		return nil, processor.NewPaymentError(processor.MPC, "TX_AUTHORIZE_FAILED", "failed to create pending MPC transaction", err)
	}

	return &processor.PaymentResult{
		Success:       true,
		TransactionID: txResp.ID,
		ProcessorRef:  txResp.ID,
		Status:        txResp.Status,
		Metadata: map[string]interface{}{
			"requires_approval": true,
			"chain":             chain,
		},
	}, nil
}

// Capture approves and executes a previously authorized transaction.
func (mp *MPCProcessor) Capture(ctx context.Context, transactionID string, amount money.Amount) (*processor.PaymentResult, error) {
	// Approve the transaction via MPC API: POST /api/v1/transactions/{id}/approve
	approveURL := fmt.Sprintf("%s/api/v1/transactions/%s/approve", mp.mpcEndpoint, transactionID)

	var txResp mpcTxResp
	err := mp.http.doJSON(ctx, http.MethodPost, approveURL, nil, &txResp)
	if err != nil {
		return nil, processor.NewPaymentError(processor.MPC, "TX_CAPTURE_FAILED", "failed to approve MPC transaction", err)
	}

	return &processor.PaymentResult{
		Success:       true,
		TransactionID: transactionID,
		ProcessorRef:  transactionID,
		Status:        txResp.Status,
	}, nil
}

// Refund signs a refund transaction via MPC (outbound transfer back to source).
func (mp *MPCProcessor) Refund(ctx context.Context, req processor.RefundRequest) (*processor.RefundResult, error) {
	// Look up the original transaction to get the source address
	origTx, err := mp.GetTransaction(ctx, req.TransactionID)
	if err != nil {
		return nil, processor.NewPaymentError(processor.MPC, "REFUND_LOOKUP_FAILED", "failed to look up original transaction for refund", err)
	}

	chain := ""
	sourceAddr := ""
	if origTx.Metadata != nil {
		if c, ok := origTx.Metadata["chain"].(string); ok {
			chain = c
		}
		if a, ok := origTx.Metadata["from"].(string); ok {
			sourceAddr = a
		}
	}

	// Create a refund transaction via MPC API
	txReqURL := fmt.Sprintf("%s/api/v1/transactions", mp.mpcEndpoint)

	var txResp mpcTxResp
	err = mp.http.doJSON(ctx, http.MethodPost, txReqURL, &mpcCreateTxReq{
		WalletID:  req.TransactionID,
		TxType:    "refund",
		Chain:     chain,
		ToAddress: sourceAddr,
		Amount:    req.Amount.MinorString(),
	}, &txResp)
	if err != nil {
		return nil, processor.NewPaymentError(processor.MPC, "REFUND_FAILED", "failed to create MPC refund transaction", err)
	}

	return &processor.RefundResult{
		Success:      true,
		RefundID:     txResp.ID,
		ProcessorRef: txResp.ID,
	}, nil
}

// GetTransaction retrieves transaction details from the MPC service.
func (mp *MPCProcessor) GetTransaction(ctx context.Context, txID string) (*processor.Transaction, error) {
	reqURL := fmt.Sprintf("%s/api/v1/transactions/%s", mp.mpcEndpoint, txID)

	var txResp mpcTxResp
	err := mp.http.doJSON(ctx, http.MethodGet, reqURL, nil, &txResp)
	if err != nil {
		return nil, processor.NewPaymentError(processor.MPC, "TX_QUERY_FAILED", "failed to query MPC transaction", err)
	}

	now := time.Now().Unix()
	return &processor.Transaction{
		ID:           txResp.ID,
		ProcessorRef: txResp.ID,
		Type:         "transfer",
		Amount:       currency.Cents(0), // MPC service stores amount as string
		Currency:     currency.Type(txResp.Chain),
		Status:       txResp.Status,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]interface{}{
			"chain":  txResp.Chain,
			"txHash": txResp.TxHash,
		},
	}, nil
}

// ValidateWebhook validates an incoming blockchain event notification.
//
// The MPC service signs every delivery as
//
//	hex(HMAC-SHA256(webhookSecret, rawBody))
//
// over the exact bytes it POSTed, and sends the digest in the
// X-Webhook-Signature header (lux/mpc pkg/api/webhook_sender.go). The digest is
// recomputed here over the raw payload for that reason: a re-marshal of the
// parsed event is a different byte string and would never reproduce it.
//
// This ingress is unauthenticated, so the signature is the whole trust anchor
// for an event that credits a balance — HIP-18 states it plainly: "Commerce
// verifies webhook signature (HMAC-SHA256)". Every path that cannot COMPLETE
// that comparison therefore refuses: an off rail, an unset secret, an absent
// signature and a malformed digest are all rejections. In particular an unset
// secret refuses rather than accepting, because "no key configured" is the one
// state in which nothing at all is being verified.
//
// The body is authenticated BEFORE it is parsed, so unverified attacker bytes
// never reach the decoder.
func (mp *MPCProcessor) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*processor.WebhookEvent, error) {
	// A rail that is turned off settles nothing, and whatever secret happens to
	// be left in its environment is not evidence that it should.
	if !mp.configured() {
		return nil, fmt.Errorf("%w: mpc rail is not configured", processor.ErrWebhookValidationFailed)
	}
	if mp.webhookSecret == "" {
		return nil, fmt.Errorf("%w: mpc webhook secret is not configured", processor.ErrWebhookValidationFailed)
	}
	if len(payload) == 0 || signature == "" {
		return nil, processor.ErrWebhookValidationFailed
	}

	mac := hmac.New(sha256.New, []byte(mp.webhookSecret))
	mac.Write(payload)
	expected := mac.Sum(nil)

	// The sender hex-encodes the digest. Decode it and compare raw bytes with
	// hmac.Equal so the comparison is constant-time and cannot be walked byte
	// by byte with a timing oracle.
	provided, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return nil, processor.ErrWebhookValidationFailed
	}
	if !hmac.Equal(provided, expected) {
		return nil, processor.ErrWebhookValidationFailed
	}

	// Parse the webhook payload from the MPC service, now that it is known to
	// have come from the MPC service.
	var event struct {
		ID        string                 `json:"id"`
		Type      string                 `json:"type"`
		Data      map[string]interface{} `json:"data"`
		Timestamp int64                  `json:"timestamp"`
		Signature string                 `json:"signature"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, processor.NewPaymentError(processor.MPC, "WEBHOOK_PARSE_FAILED", "failed to parse webhook payload", err)
	}

	return &processor.WebhookEvent{
		ID:        event.ID,
		Type:      event.Type,
		Processor: processor.MPC,
		Data:      event.Data,
		Timestamp: event.Timestamp,
	}, nil
}

// IsAvailable checks if the MPC and KMS services are reachable.
//
// The probe rides the SAME wire keygen does, which is the point: a rail that
// health-checked HTTP while minting over ZAP would report a signer it does not
// actually use, and would answer "available" for a wire that is down.
func (mp *MPCProcessor) IsAvailable(ctx context.Context) bool {
	if !mp.configured() {
		return false
	}

	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return mp.transport.Health(healthCtx) == nil
}

// chainForCurrency maps a currency type to its primary chain.
func chainForCurrency(c currency.Type) string {
	switch c {
	case currency.BTC:
		return "bitcoin"
	case currency.ETH:
		return "ethereum"
	case currency.Type("sol"):
		return "solana"
	case currency.Type("matic"):
		return "polygon"
	case currency.Type("avax"):
		return "avalanche"
	case currency.Type("lux"):
		return "lux"
	case currency.Type("arb"):
		return "arbitrum"
	case currency.Type("op"):
		return "optimism"
	case currency.Type("base"):
		return "base"
	case currency.Type("usdc"), currency.Type("usdt"):
		return "ethereum" // default to Ethereum for stablecoins
	default:
		return "ethereum"
	}
}

// Ensure MPCProcessor implements CryptoProcessor.
var _ processor.CryptoProcessor = (*MPCProcessor)(nil)
