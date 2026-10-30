// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package gateway is the USDC authorization top-up rail: a caller signs an
// EIP-3009 transferWithAuthorization in its own wallet, made out from that
// wallet to Hanzo's treasury; the rail checks it, submits it to the USDC
// contract with Hanzo's relayer paying the gas, and credits the caller's
// account once the transfer is confirmed on chain.
//
// Three calls, no human, no gas, no balance held anywhere but the payer's own
// wallet:
//
//	GET  /v1/billing/crypto/gateway      Terms: where to pay, on which networks,
//	                                     the token's EIP-712 domain and types to
//	                                     sign, the limits, and a nonce issued to
//	                                     the caller.
//	POST /v1/billing/crypto/gateway      Submit the signed payment.
//	GET  /v1/billing/crypto/gateway/:id  Read it back: pending → confirming →
//	                                     succeeded, or refused / failed.
//
// WHO IS CREDITED is the authenticated caller — their org, their billing
// subject, their books — and nothing in the payment can say otherwise. The
// payment's `from` is proven by its signature and names who PAID; the caller
// names who is CREDITED; and the nonce binds the two (bind): it is issued to
// one caller by Terms, so a payment captured on its way here cannot be
// presented by anyone else. Without that, a signed payment would be a bearer
// instrument redeemable by the first org to present it.
//
// CREDIT ON CONFIRMATION, never before. The authorization moves the payer's
// USDC to the treasury in one contract call, and the USDC contract is the only
// authority on whether it did: the rail credits when the chain shows the
// authorization used (AuthorizationUsed(from, nonce)) in a successful
// transaction that moved exactly the signed value from the payer to the
// treasury, that transaction Confirmations blocks deep. Who submitted it does
// not matter — the relayer, the payer, anyone holding the signature — because
// the signature fixes the recipient and the amount. So nothing is ever owed
// back: there is no credit to reverse.
//
// EXACTLY ONCE on the authorization nonce. The record id is (network, from,
// nonce); the USDC contract refuses a second use of (from, nonce); every state
// change is a swap over the row read; and the credit is keyed on the record in
// the host ledger and on a deterministic receipt key.
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
)

// Limits on one payment, in ledger cents. Constants, not configuration: a
// knob on a money path is a knob someone eventually sets to zero, and moving
// either is a reviewed change.
const (
	MinCents = 100     // $1.00
	MaxCents = 100_000 // $1,000.00

	// MinValidity is the shortest an authorization may have left when it is
	// presented: the relayer's transaction has to land before validBefore.
	MinValidity = 5 * time.Minute
	// MaxValidity is the furthest validBefore may be: how long a signed payment
	// is followed.
	MaxValidity = 24 * time.Hour
	// MaxAge is the furthest back validAfter may be: how far before it was
	// presented the chain is searched for a use of it the rail did not submit.
	MaxAge = time.Hour

	// FinalCents is the payment from which a credit waits for the chain's
	// FINALIZED head; below it, the SAFE head.
	FinalCents = 10_000 // $100.00

	// MaxOpen is how many payments one org may have pending at once: each can
	// cost the relayer gas, and an account costs nothing to make.
	MaxOpen = 5
	// MaxRefused is how many refused payments one org may have watched at
	// once: each is polled on chain until it expires, so an org cannot turn
	// free refusals into unbounded reads.
	MaxRefused = 20
	// DayCents is the most one org may present in a UTC day.
	DayCents = 500_000 // $5,000.00
	// PerMinute is how many terms and how many submissions one org may ask for
	// a minute, each.
	PerMinute = 30
)

// readyAge is how long a network's self-check is trusted before it is run
// again; failAge how long a failed one is remembered.
const (
	readyAge = 10 * time.Minute
	failAge  = 30 * time.Second
)

// Service is the rail. One per process (Default); unarmed, it refuses
// everything with ReasonClosed.
type Service struct {
	cfg   Config
	links map[string]*link // by CAIP-2 network
	now   func() time.Time
	every time.Duration

	// relay serialises this process's relayer sends; the relayer lease (lease)
	// serialises them across replicas.
	relay sync.Mutex
	// self names this process as a lease holder.
	self string
	// rate is the per-org request budget (PerMinute).
	rate limiter

	run    sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// New builds the rail from its configuration. Nothing is dialled here: each
// network is checked against its chain the first time it is used (ready).
func New(cfg Config) *Service {
	s := &Service{cfg: cfg, links: map[string]*link{}, now: time.Now, every: time.Minute, self: newSelf()}
	for _, c := range cfg.Chains {
		s.links[c.ID] = &link{chain: c}
	}
	return s
}

var current atomic.Pointer[Service]

func init() { current.Store(New(Config{})) }

// Default is the process's rail (Bootstrap installs it; unarmed until then).
func Default() *Service { return current.Load() }

// SetDefault installs the process's rail.
func SetDefault(s *Service) { current.Store(s) }

// Armed reports whether a treasury, a relayer and a network are configured.
func (s *Service) Armed() bool { return s != nil && s.cfg.Armed() }

// Config is the rail's configuration.
func (s *Service) Config() Config { return s.cfg }

// Network names which money the rail moves: "mainnet" or "testnet", or "" when
// it is not armed.
func (s *Service) Network() string {
	switch {
	case !s.Armed():
		return ""
	case s.cfg.Testnet:
		return "testnet"
	}
	return "mainnet"
}

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

// Reasons this rail states. A refusal the USDC contract made is named by what
// it refused for (insufficient_balance, invalid_signature, blocked, …).
const (
	ReasonClosed      = "rail_closed"
	ReasonBooks       = "books_mismatch"
	ReasonUnavailable = "network_unavailable"
	ReasonRelayer     = "relayer_unfunded"
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
	ReasonStale       = "authorization_too_old"
	ReasonUnbound     = "nonce_not_issued"
	ReasonSignature   = "invalid_signature"
	ReasonBalance     = "insufficient_balance"
	ReasonBlocked     = "blocked"
	ReasonCanceled    = "canceled"
	ReasonUnaccounted = "unaccounted"
	ReasonOpen        = "too_many_open_payments"
	ReasonDaily       = "daily_limit"
	ReasonRate        = "rate_limited"
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

var errClosed = shut(ReasonClosed, "USDC top-up by signed authorization is not available on this deployment. Use a deposit address, a card, bank transfer or wire.", nil)

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
	case test && !s.cfg.Testnet:
		return shut(ReasonBooks, "This account transacts in test mode, and this rail moves real USDC on mainnet, which cannot fund test books. Top up a live account.", nil)
	case !test && s.cfg.Testnet:
		return shut(ReasonBooks, "This rail moves testnet USDC, which cannot fund a live balance. Use a card, bank transfer or wire.", nil)
	}
	return nil
}
