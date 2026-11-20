// Package nano implements a Nano (XNO) payment processor for Commerce: the
// `nano:mainnet` network with the `exact` scheme. A charge asks the buyer to
// send an exact raw amount to the merchant's account; a payment is accepted
// only when a node the operator configures reports that send as confirmed.
//
// It holds no key and no seed and never sends money. It is off until an
// account names a recipient, a node URL and a claim directory.
package nano

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hanzoai/money"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/nano/nanoverify"
)

const (
	// Network and Scheme name what this processor settles, in the terms other
	// x402-style implementations use for Nano.
	Network = "nano:mainnet"
	Scheme  = "exact"

	defaultTimeout = 30 * time.Second
)

// Config holds Nano configuration. All three are required; there is no
// default node, because which node to trust is the operator's decision.
type Config struct {
	// RPCURL is a Nano node's JSON RPC endpoint, read-only use.
	RPCURL string
	// Recipient is the merchant's nano_ account.
	Recipient string
	// ClaimDir is a directory that persists across restarts; it records which
	// order each send block settled, so a block cannot settle two orders.
	ClaimDir string
}

// Provider implements PaymentProcessor and CryptoProcessor for Nano.
type Provider struct {
	*processor.BaseProcessor
	node      nanoverify.Node
	recipient string
	claims    nanoverify.FileClaims
}

// NewProvider creates a Nano provider. It is configured only when the
// recipient passes its checksum and the node URL and claim directory are set.
func NewProvider(cfg Config) *Provider {
	p := &Provider{BaseProcessor: processor.NewBaseProcessor(processor.Nano, supportedCurrencies())}
	p.apply(cfg)
	return p
}

func init() {
	processor.Register(NewProvider(Config{}))
}

func (p *Provider) apply(cfg Config) {
	p.node = nanoverify.Node{URL: strings.TrimSpace(cfg.RPCURL), Client: &http.Client{Timeout: defaultTimeout}}
	p.claims = nanoverify.FileClaims{Dir: strings.TrimSpace(cfg.ClaimDir)}
	p.recipient = ""
	if a, err := nanoverify.ParseAddress(cfg.Recipient); err == nil {
		p.recipient = a
	}
	p.SetConfigured(p.recipient != "" && p.node.URL != "" && p.claims.Dir != "")
}

// Reconfigure reads the "default" nano account from the routing policy: the
// node URL, the recipient and the claim directory, all plain settings.
func (p *Provider) Reconfigure() {
	a := processor.AccountOf(processor.Nano, "default")
	p.apply(Config{RPCURL: a.Setting("rpcUrl"), Recipient: a.Setting("recipient"), ClaimDir: a.Setting("claimDir")})
}

func supportedCurrencies() []currency.Type {
	return []currency.Type{currency.XNO}
}

func (p *Provider) ready() error {
	if p.recipient == "" || p.node.URL == "" || p.claims.Dir == "" {
		return processor.NewPaymentError(processor.Nano, "NOT_CONFIGURED", "nano recipient, rpcUrl and claimDir must all be set", nil)
	}
	return nil
}

// ---------------------------------------------------------------------------
// PaymentProcessor
// ---------------------------------------------------------------------------

// Charge prices the order in raw and returns a nano: payment URI. The amount
// carries a per-order tag below one minor unit, so the exact raw amount is
// what binds a send to this order. TransactionID is that raw amount; pass it
// back to GetTransaction or in the webhook payload.
func (p *Provider) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}
	if req.Currency != currency.XNO {
		return nil, processor.ErrCurrencyNotSupported
	}
	if err := p.ready(); err != nil {
		return nil, err
	}
	reference := uuid.New().String()
	raw, err := nanoverify.ExpectedRaw(int64(req.Amount), currency.XNO.Decimals(), reference)
	if err != nil {
		return nil, processor.NewPaymentError(processor.Nano, "AMOUNT", err.Error(), err)
	}
	return &processor.PaymentResult{
		Success:       true,
		TransactionID: raw.String(),
		ProcessorRef:  reference,
		Status:        "pending",
		Metadata: map[string]interface{}{
			"nano_uri":   "nano:" + p.recipient + "?amount=" + raw.String(),
			"network":    Network,
			"scheme":     Scheme,
			"recipient":  p.recipient,
			"amount_raw": raw.String(),
			"reference":  reference,
		},
	}, nil
}

// Authorize is not supported: a Nano send is final.
func (p *Provider) Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	return nil, processor.NewPaymentError(processor.Nano, "NOT_SUPPORTED", "authorize not supported for on-chain payments", nil)
}

// Capture is not supported.
func (p *Provider) Capture(ctx context.Context, transactionID string, amount money.Amount) (*processor.PaymentResult, error) {
	return nil, processor.NewPaymentError(processor.Nano, "NOT_SUPPORTED", "capture not supported for on-chain payments", nil)
}

// Refund is not supported: this processor holds no key and cannot send.
func (p *Provider) Refund(ctx context.Context, req processor.RefundRequest) (*processor.RefundResult, error) {
	return &processor.RefundResult{
		Success:      false,
		ErrorMessage: "refunds not supported: the nano processor holds no key",
	}, processor.NewPaymentError(processor.Nano, "NOT_SUPPORTED", "refunds not supported for on-chain payments", nil)
}

// GetTransaction looks for a confirmed send of exactly txID raw to the
// recipient. It reports "pending" until one exists. It does not claim the
// block; ValidateWebhook does, once, when the order is booked.
func (p *Provider) GetTransaction(ctx context.Context, txID string) (*processor.Transaction, error) {
	if err := p.ready(); err != nil {
		return nil, err
	}
	want, err := nanoverify.ParseRaw(txID)
	if err != nil {
		return nil, processor.ErrTransactionNotFound
	}
	hash, err := p.node.FindSend(ctx, p.recipient, want, 0)
	if err != nil {
		return nil, fmt.Errorf("nano find send: %w", err)
	}
	tx := &processor.Transaction{ID: txID, Type: "transfer", Currency: currency.XNO, Status: "pending"}
	if minor, err := nanoverify.MinorOf(want, currency.XNO.Decimals()); err == nil {
		tx.Amount = currency.Cents(minor)
	}
	if hash == "" {
		return tx, nil
	}
	paid, err := p.verify(ctx, hash, want)
	if err != nil {
		return tx, nil
	}
	tx.ProcessorRef = paid.Hash
	tx.Status = "confirmed"
	tx.Metadata = map[string]interface{}{"block": paid.Hash, "payer": paid.Payer, "amount_raw": paid.Amount.String()}
	return tx, nil
}

func (p *Provider) verify(ctx context.Context, hash string, want *big.Int) (*nanoverify.Paid, error) {
	info, err := p.node.BlockInfo(ctx, hash)
	if errors.Is(err, nanoverify.ErrBlockNotFound) {
		return nil, processor.ErrTransactionNotFound
	}
	if err != nil {
		return nil, err
	}
	return nanoverify.CheckSend(hash, info, p.recipient, want)
}

// webhookPayload is what a caller posts once the buyer's wallet has sent:
// the send block's hash and the order's TransactionID (its raw amount).
type webhookPayload struct {
	Block         string `json:"block"`
	TransactionID string `json:"transactionId"`
}

// ValidateWebhook accepts a payment only after reading the block from the
// node: a confirmed send of exactly the order's raw amount to the recipient.
// It then claims the block for the order on disk. The same block for the
// same order again is reported as payment.duplicate; for another order it is
// refused, before and after a restart.
func (p *Provider) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*processor.WebhookEvent, error) {
	if err := p.ready(); err != nil {
		return nil, err
	}
	var in webhookPayload
	if err := json.Unmarshal(payload, &in); err != nil {
		return nil, processor.ErrWebhookValidationFailed
	}
	if signature != "" {
		in.Block = signature
	}
	want, err := nanoverify.ParseRaw(in.TransactionID)
	if err != nil {
		return nil, processor.ErrWebhookValidationFailed
	}
	paid, err := p.verify(ctx, in.Block, want)
	if err != nil {
		return nil, err
	}
	dup, err := p.claims.Claim(paid.Hash, in.TransactionID)
	if err != nil {
		return nil, processor.NewPaymentError(processor.Nano, "REPLAY", err.Error(), err)
	}
	kind := "payment.confirmed"
	if dup {
		kind = "payment.duplicate"
	}
	return &processor.WebhookEvent{
		ID:        paid.Hash,
		Type:      kind,
		Processor: processor.Nano,
		Data: map[string]interface{}{
			"block":          paid.Hash,
			"payer":          paid.Payer,
			"amount_raw":     paid.Amount.String(),
			"transaction_id": in.TransactionID,
			"network":        Network,
		},
		Timestamp: time.Now().Unix(),
	}, nil
}

// ---------------------------------------------------------------------------
// CryptoProcessor
// ---------------------------------------------------------------------------

// GenerateAddress returns the one configured recipient; orders are told apart
// by their exact raw amount, not by address.
func (p *Provider) GenerateAddress(ctx context.Context, customerID string, chain string) (processor.Wallet, error) {
	if chain != "nano" {
		return processor.Wallet{}, processor.NewPaymentError(processor.Nano, "UNSUPPORTED_CHAIN", fmt.Sprintf("chain %s not supported, use nano", chain), nil)
	}
	if p.recipient == "" {
		return processor.Wallet{}, processor.NewPaymentError(processor.Nano, "NO_RECIPIENT", "recipient address not configured", nil)
	}
	return processor.Wallet{Address: p.recipient}, nil
}

// GetBalance reads an account's balance, floored to XNO's priced minor unit.
func (p *Provider) GetBalance(ctx context.Context, address string, chain string) (*processor.Balance, error) {
	if chain != "nano" {
		return nil, processor.NewPaymentError(processor.Nano, "UNSUPPORTED_CHAIN", fmt.Sprintf("chain %s not supported", chain), nil)
	}
	bal, rec, err := p.node.Balance(ctx, address)
	if err != nil {
		return nil, err
	}
	d := currency.XNO.Decimals()
	avail, err := nanoverify.MinorOf(bal, d)
	if err != nil {
		return nil, err
	}
	pend, err := nanoverify.MinorOf(rec, d)
	if err != nil {
		return nil, err
	}
	return &processor.Balance{Available: currency.Cents(avail), Pending: currency.Cents(pend), Currency: currency.XNO}, nil
}

// EstimateFee is zero: Nano has no transaction fee.
func (p *Provider) EstimateFee(ctx context.Context, req processor.PaymentRequest) (currency.Cents, error) {
	return 0, nil
}

// SupportedChains returns the chains supported by this processor.
func (p *Provider) SupportedChains() []string {
	return []string{"nano"}
}

// Compile-time interface checks.
var (
	_ processor.PaymentProcessor = (*Provider)(nil)
	_ processor.CryptoProcessor  = (*Provider)(nil)
)
