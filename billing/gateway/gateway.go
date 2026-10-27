// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package gateway is the Circle Gateway top-up rail: a caller presents a USDC
// payment, signed as an EIP-3009 authorization against their Gateway balance
// and made out to Hanzo's treasury; the rail checks it, hands it to Gateway,
// and credits the caller's account the moment Gateway has locked the money.
//
// Three calls, no human, no docs:
//
//	GET  /v1/billing/crypto/gateway      Terms: where to pay, on which networks,
//	                                     the EIP-712 domain and types to sign,
//	                                     the limits, and a nonce issued to the
//	                                     caller.
//	POST /v1/billing/crypto/gateway      Submit the signed payment.
//	GET  /v1/billing/crypto/gateway/:id  Read it back: pending → accepted →
//	                                     settled, or refused / failed.
//
// WHO IS CREDITED is the authenticated caller — their org, their billing
// subject, their books — and nothing in the payment can say otherwise. The
// payment's `from` is proven by its signature and names who PAID; the caller
// names who is CREDITED; and the nonce binds the two (bind): it is issued to
// one caller by Terms, so a payment captured on its way here cannot be
// presented by anyone else. Without that, a signed payment would be a bearer
// instrument redeemable by the first org to present it.
//
// CREDIT ON ACCEPT. Gateway's settle answers success only after it has
// verified the signature and locked the payer's balance; from then the payer
// cannot spend that USDC through Gateway, and the only exit — a trustless
// onchain withdrawal — waits out a 7-day delay that is exactly the shortest
// validity Gateway accepts (minValiditySeconds), so a batch lands before the
// lock can be escaped. Crediting at settlement instead would make every agent
// wait for a batch it cannot see. Credit on a lock is capped: a payer holds
// at most MaxCents of it in total (Hold), and past that a payment is credited
// when Gateway confirms it onchain. The residual risk is Gateway failing a
// payment it accepted: the reconciler then reverses the credit (the balance
// may go below zero, and the spend gate refuses from there), so the loss is
// bounded by what was spent between acceptance and failure, at most MaxCents
// per payer.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/providers/circle"
)

// Limits on one payment, in ledger cents. Constants, not configuration: a
// knob on a money path is a knob someone eventually sets to zero, and moving
// either is a reviewed change.
const (
	MinCents = 100     // $1.00
	MaxCents = 100_000 // $1,000.00

	// MaxValidity is the longest an authorization may stay open. Gateway sets
	// the floor (minValiditySeconds, 7 days today); this sets the ceiling, so a
	// payment the rail has not yet seen settle is followed for a bounded time.
	MaxValidity = 30 * 24 * time.Hour
)

// fallbackValidity is the floor used if Gateway ever stops stating one: the
// @circle-fin/x402-batching SDK's GATEWAY_MIN_AUTH_VALIDITY_SECONDS, which is
// what Gateway states today. Erring long only refuses payments; erring short
// would accept payments Gateway refuses.
const fallbackValidity = 7 * 24 * time.Hour

// termsAge is how long Gateway's network table is trusted before it is read
// again; failAge how long a failed read is remembered before the next try.
const (
	termsAge = 10 * time.Minute
	failAge  = 30 * time.Second
)

// Service is the rail. One per process (Default); unarmed, it refuses
// everything with ReasonClosed.
type Service struct {
	cfg    circle.Config
	client *circle.Client
	now    func() time.Time
	every  time.Duration

	mu     sync.Mutex
	nets   []network
	read   time.Time
	ferr   error
	failed time.Time
	flight chan struct{}

	run    sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// New builds the rail from its configuration and a Gateway client for
// cfg.Network.
func New(cfg circle.Config, client *circle.Client) *Service {
	return &Service{cfg: cfg, client: client, now: time.Now, every: time.Minute}
}

var current atomic.Pointer[Service]

func init() { current.Store(New(circle.Config{}, nil)) }

// Default is the process's rail (Bootstrap installs it; unarmed until then).
func Default() *Service { return current.Load() }

// SetDefault installs the process's rail.
func SetDefault(s *Service) { current.Store(s) }

// Armed reports whether a treasury is configured.
func (s *Service) Armed() bool { return s != nil && s.cfg.Armed() && s.client != nil }

// Config is the rail's configuration (treasury address and network): plain
// values, nothing secret.
func (s *Service) Config() circle.Config { return s.cfg }

// ── refusals ───────────────────────────────────────────────────────────────

// Error is a refusal, carrying the HTTP status both transports answer it with
// (commerce's own route and cloud's) and a machine-readable reason, so the two
// cannot map one refusal two ways.
type Error struct {
	Status int
	Reason string
	Msg    string
	cause  error
}

func (e *Error) Error() string { return e.Msg }
func (e *Error) Unwrap() error { return e.cause }

// StatusOf is the HTTP status err answers with: a refusal's own, else 500.
func StatusOf(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.Status
	}
	return http.StatusInternalServerError
}

// ReasonOf is err's machine-readable reason, or "" for an unexpected error.
func ReasonOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Reason
	}
	return ""
}

// Reasons this rail states itself. A refusal Gateway made carries Gateway's
// own errorReason (insufficient_balance, invalid_signature, …) unchanged.
const (
	ReasonClosed      = "rail_closed"
	ReasonBooks       = "books_mismatch"
	ReasonUnavailable = "gateway_unavailable"
	ReasonNetwork     = "unsupported_network"
	ReasonPayload     = "invalid_payload"
	ReasonRecipient   = "recipient_mismatch"
	ReasonCents       = "amount_not_whole_cents"
	ReasonLow         = "amount_below_minimum"
	ReasonHigh        = "amount_above_maximum"
	ReasonEarly       = "authorization_not_yet_valid"
	ReasonExpired     = "authorization_expired"
	ReasonShort       = "authorization_validity_too_short"
	ReasonLong        = "authorization_validity_too_long"
	ReasonUnbound     = "nonce_not_issued"
	ReasonSignature   = "invalid_signature"
	ReasonConflict    = "nonce_conflict"
	ReasonMissing     = "not_found"
	ReasonCaller      = "unauthenticated"
	ReasonRejected    = "rejected"
)

func refuse(status int, reason, format string, args ...any) *Error {
	return &Error{Status: status, Reason: reason, Msg: fmt.Sprintf(format, args...)}
}

func invalid(reason, format string, args ...any) *Error {
	return refuse(http.StatusBadRequest, reason, format, args...)
}

func shut(reason, msg string, cause error) *Error {
	return &Error{Status: http.StatusServiceUnavailable, Reason: reason, Msg: msg, cause: cause}
}

var errClosed = shut(ReasonClosed, "USDC top-up through Circle Gateway is not available on this deployment. Use a card, bank transfer or wire.", nil)

// errMissing is ONE answer for an id that names nothing and an id that names
// another caller's payment, so a guessed id cannot confirm a stranger's.
var errMissing = refuse(http.StatusNotFound, ReasonMissing, "no such payment for this account")

// caller checks the rail is open to this caller: armed, an org and a payer
// resolved, and the org's books the ones this network's money can fund.
//
// Testnet USDC is free from a faucet, so it may only ever fund TEST books, and
// mainnet USDC is real money, so it may only fund LIVE books. The org's own
// TestMode is the one authority for which books it keeps (organization.TestMode);
// a caller whose books are the other kind is refused before anything is
// signed or sent, never credited into books it cannot spend.
func (s *Service) caller(org *organization.Organization, payer string) error {
	if org == nil || payer == "" {
		return refuse(http.StatusUnauthorized, ReasonCaller, "authentication required")
	}
	if !s.Armed() {
		return errClosed
	}
	switch test := org.TestMode(); {
	case test && s.cfg.Network == circle.Mainnet:
		return shut(ReasonBooks, "This account transacts in test mode, and this rail settles real USDC on mainnet, which cannot fund test books. Top up a live account.", nil)
	case !test && s.cfg.Network == circle.Testnet:
		return shut(ReasonBooks, "This rail settles testnet USDC, which cannot fund a live balance. Use a card, bank transfer or wire.", nil)
	}
	return nil
}
