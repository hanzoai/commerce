package mpc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
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

	// http carries signing, which has no wire but this one: mpcd defines an
	// MPC-API ZAP surface with a sign opcode but never starts it — pkg/api
	// StartZAP has no callers — so nothing in the fleet listens for it on any
	// port or socket. When no ZAP address is configured this is also the
	// Transport above, so the HTTP plumbing exists once.
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
// not consulted here. Selecting ZAP moves keygen and health onto that wire;
// signing has no ZAP surface running anywhere in the fleet to move to, so a
// rail without an HTTP endpoint is still a rail that can mint a custody address
// and never spend from it.
func (mp *MPCProcessor) configured() bool {
	return mp.kmsEndpoint != "" && mp.mpcEndpoint != ""
}

// MPCSupportedCurrencies returns cryptocurrencies supported by MPC.
func MPCSupportedCurrencies() []currency.Type {
	// SOL AND TON ARE NOT HERE, and the reason is no longer the one this
	// comment carried for months. It said the custody fleet ran only the
	// secp256k1 ceremony, so EDDSAPubKey came back empty and mpcd never emitted
	// sol_address — and concluded that restoring sol was "protocol work, not a
	// list edit". That ceremony now runs: a live keygen returns a 32-byte
	// eddsa_pub_key, both sol_address and ton_address, and /sign answers a
	// 64-byte Ed25519 signature. SupportedChains lists both today.
	//
	// What keeps the COINS off this list is a different limit that still holds:
	// a native coin cannot be priced here. depositwatch credits at a fixed peg
	// and holds no oracle, so SOL and TON have no value this rail could put on a
	// balance — the same reason ETH is absent as a coin while ethereum is a
	// chain. Both chains carry a dollar-pegged TOKEN (SPL USDC, jetton USDT),
	// and that is what is credited on them.
	//
	// The distinction to keep: a CHAIN is offerable when an address can be
	// minted and its deposits watched; a COIN is creditable when it can be
	// priced. They are different questions and this list answers the second.
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

// addrKind is WHICH of a keygen reply's addresses a chain is paid at.
//
// One keygen yields several addresses off two keys, and picking the wrong one
// does not fail — it hands out a well-formed address on the wrong network, which
// a buyer then pays into and nobody can ever spend.
type addrKind int

const (
	addrEVM addrKind = iota // secp256k1, the one address every EVM chain shares
	addrBTC
	addrSOL
	addrTON
)

// mintChains is the ONE declaration of what this signer can mint, and of where
// each chain's address comes from.
//
// It is one table and not two lists because both directions have already shipped
// a live defect. GenerateAddress used to switch on chain with an EVM DEFAULT, so
// a chain nobody had mapped silently received the secp256k1 address: TON would
// have been handed an 0x string no TON wallet can pay, and money sent to it is
// gone. Meanwhile SupportedChains was a separate literal that omitted solana
// long after the Ed25519 ceremony started working, so a chain that COULD be
// minted and WAS being watched was never offered to anyone.
//
// A chain absent here is refused rather than defaulted. That is the safety
// property: this table is the only way to offer a chain, and adding one forces
// you to say which key its address is derived from.
//
// XRPL is deliberately absent and must stay absent. Its deposits are POOLED —
// the account is configured, never minted — so the mint path does not consult
// this signer about it at all.
var mintChains = map[string]addrKind{
	"bitcoin": addrBTC,
	"solana":  addrSOL,
	"ton":     addrTON,

	"ethereum":  addrEVM,
	"polygon":   addrEVM,
	"arbitrum":  addrEVM,
	"optimism":  addrEVM,
	"base":      addrEVM,
	"avalanche": addrEVM,
	"bsc":       addrEVM,
	"lux":       addrEVM,
	"zoo":       addrEVM,
}

// SupportedChains returns blockchain networks supported by MPC, sorted so the
// picker's order is a property of the table rather than of map iteration.
func (mp *MPCProcessor) SupportedChains() []string {
	out := make([]string, 0, len(mintChains))
	for c := range mintChains {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// Type returns the processor type.
func (mp *MPCProcessor) Type() processor.ProcessorType {
	return processor.MPC
}

// errNotAWallet is why five of this processor's methods refuse instead of
// calling out.
//
// They used to POST to {mpcEndpoint}/api/v1/transactions and expect the signer
// to build, sign and broadcast a chain transaction. mpcd serves no such route:
// its internal surface is /healthz, /keys, /backup, /keygen and /sign, and that
// path answers 404. Nor could it be repointed. The route on mpcd's OTHER server
// that is spelled similarly (POST /v1/transactions on the node-0 dashboard) is
// not a builder either — it records a row and, if policy approves, broadcasts a
// raw_tx the CALLER already built.
//
// So there is nothing to point at, and there should not be: a threshold signer
// that knew about chains, tokens and amounts would be a wallet, and the whole
// reason custody is safe is that the component holding the key shares can only
// ever see a digest. Building and broadcasting is commerce's job and lives in
// billing/custody, which reads the chain with commerce's own clients and asks
// this processor for signatures through Sign.
//
// These methods refuse loudly rather than being deleted because the
// CryptoProcessor interface requires them, and rather than being stubbed
// "successful" because a payment rail that reports success without moving money
// is the worst of the three options.
var errNotAWallet = errors.New(
	"mpc: the MPC fleet is a threshold signer, not a wallet — it signs digests and does not build, submit or track chain transactions; " +
		"spending from custody goes through billing/custody, which drafts with commerce's own chain clients and signs via MPCProcessor.Sign")

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
	TONAddress string `json:"ton_address"`
	Error      string `json:"error"`
	ErrorCode  string `json:"error_code"`
}

// address returns the one address of KIND, or "" when the reply carried none.
//
// Empty is a refusal and never a fallback: an address is where a customer's
// money goes, so answering with a different chain's address would be worse than
// answering with nothing. GenerateAddress turns "" into NO_ADDRESS.
func (r mpcKeygenResp) address(k addrKind) string {
	switch k {
	case addrBTC:
		return r.BTCAddress
	case addrSOL:
		return r.SOLAddress
	case addrTON:
		return r.TONAddress
	case addrEVM:
		return r.EVMAddress
	}
	return ""
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
	kind, ok := mintChains[chain]
	if !ok {
		// Refuse a chain this signer does not declare, rather than reaching for
		// the EVM address that used to sit here as the default.
		return processor.Wallet{}, processor.NewPaymentError(processor.MPC, "UNSUPPORTED_CHAIN",
			fmt.Sprintf("MPC signer mints no address for chain %s", chain), nil)
	}
	addr := resp.address(kind)
	if addr == "" {
		return processor.Wallet{}, processor.NewPaymentError(processor.MPC, "NO_ADDRESS", fmt.Sprintf("MPC keygen did not return address for chain %s", chain), nil)
	}
	return processor.Wallet{Address: addr, ID: resp.WalletID}, nil
}

// GetBalance refuses, because the signer does not hold balances.
//
// It used to GET {mpcEndpoint}/api/v1/wallets/{address}/addresses — a route
// that does not exist — decode the 404 into a map, discard the map, and return
// zero. So it reported "0" for every funded address in the fleet and reported
// it as a SUCCESS on the path where the call failed. A balance of zero is a
// perfectly ordinary answer, which is what made the lie invisible.
//
// The chain is the only thing that knows a balance. billing/husdindex reads
// ERC-20 balances over eth_call and the depositwatch readers read the rest;
// asking the signer was never going to work.
func (mp *MPCProcessor) GetBalance(ctx context.Context, address string, chain string) (*processor.Balance, error) {
	return nil, processor.NewPaymentError(processor.MPC, "NOT_A_WALLET",
		"MPC signer holds no balances; read the chain", errNotAWallet)
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

// Charge refuses. Moving money out of custody is billing/custody.Sweep, which
// drafts the transaction against the chain, asks Sign for a signature over the
// digest, and broadcasts with commerce's own client.
func (mp *MPCProcessor) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	return nil, processor.NewPaymentError(processor.MPC, "NOT_A_WALLET",
		"MPC signer cannot build or broadcast a transaction", errNotAWallet)
}

// Authorize refuses. The policy engine that held pending transactions lives on
// mpcd's dashboard server, is not part of the signing surface, and was never
// reachable from the endpoint this rail is configured with.
func (mp *MPCProcessor) Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	return nil, processor.NewPaymentError(processor.MPC, "NOT_A_WALLET",
		"MPC signer cannot hold a pending transaction", errNotAWallet)
}

// Capture refuses. There is nothing to approve: Authorize creates nothing.
func (mp *MPCProcessor) Capture(ctx context.Context, transactionID string, amount money.Amount) (*processor.PaymentResult, error) {
	return nil, processor.NewPaymentError(processor.MPC, "NOT_A_WALLET",
		"MPC signer holds no transaction to approve", errNotAWallet)
}

// Refund refuses.
//
// A refund is a spend back to the payer, which is the same problem as a sweep
// with a different destination — so when one is wanted it belongs in
// billing/custody with the payer's address as Transfer.To, not here. It is not
// wired to that yet, and saying so is better than the previous shape, which
// asked a 404 for the original transaction and reported the failure as a
// lookup problem.
func (mp *MPCProcessor) Refund(ctx context.Context, req processor.RefundRequest) (*processor.RefundResult, error) {
	return nil, processor.NewPaymentError(processor.MPC, "NOT_A_WALLET",
		"MPC signer cannot build or broadcast a refund", errNotAWallet)
}

// GetTransaction refuses. The signer keeps signing sessions, not transactions;
// what happened to a transaction is a question for the chain, and for the
// intent record commerce already holds.
func (mp *MPCProcessor) GetTransaction(ctx context.Context, txID string) (*processor.Transaction, error) {
	return nil, processor.NewPaymentError(processor.MPC, "NOT_A_WALLET",
		"MPC signer does not track transactions", errNotAWallet)
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

// Ensure MPCProcessor implements CryptoProcessor.
var _ processor.CryptoProcessor = (*MPCProcessor)(nil)
