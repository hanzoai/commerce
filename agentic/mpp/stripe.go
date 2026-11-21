// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// This file is the stripe method of the charge intent, draft-stripe-charge-00: the
// buyer's agent issues a Shared Payment Token for the challenged amount, the server
// creates and confirms a Stripe PaymentIntent on it, and the receipt names that
// PaymentIntent. The server side (Stripe, Seller) and the client side (Wallet) are
// here; what charges the token is the Seller the host passes, and Commerce is the one
// that settles it through the api/billing checkout.

var (
	currencyShape = regexp.MustCompile(`^[a-z]{3}$`)
	tokenShape    = regexp.MustCompile(`^spt_[A-Za-z0-9_]{1,250}$`)
)

// Profile is the seller's part of a stripe request, its methodDetails (section 6.2):
// the Stripe Business Network profile the payer's token is issued to, and the payment
// method types the seller can process.
type Profile struct {
	Network string
	Types   []string
	// Metadata is key-value context forwarded to Stripe.
	Metadata map[string]string
	// Decimals is the number of decimal places of the amount's currency. The draft does
	// not define it; the method's reference documentation lists it as a request member
	// and clients that follow that documentation read it, while a client that follows the
	// draft ignores it as an unknown member (section 9.2.1). Nil leaves it out.
	Decimals *int
}

// Sale is one shared payment token to charge.
type Sale struct {
	Caller Caller
	// Token is the payer's shared payment token, spt_...
	Token string
	// Amount and Currency are the challenge's: base units and a lowercase ISO code.
	Amount      string
	Currency    string
	Description string
	// External is the merchant's reference from the request.
	External string
	// Challenge is the id of the challenge being settled, which the PaymentIntent
	// carries as metadata (section 9).
	Challenge string
	// Key is the idempotency key the draft derives from the challenge and the token.
	Key string
	// Metadata is the request's methodDetails.metadata with challenge_id added.
	Metadata map[string]string
	// Until is the instant the Seller may forget what it recorded for the challenge.
	Until time.Time
}

// Seller is the merchant side of the stripe method.
type Seller interface {
	// Profile is the seller's methodDetails for the quote, or ErrSkip (or nil) when the
	// seller cannot take it.
	Profile(ctx context.Context, c Caller, q Quote) (*Profile, error)
	// Take creates and confirms the PaymentIntent on the token and returns its
	// reference, once it has succeeded (section 9). A refusal is a *Problem: Failed for
	// a token or a card that did not pay, Action when the payment needs a further step
	// from the payer, Insufficient when the token cannot cover the amount. An error that
	// is not a *Problem says the outcome is not known.
	Take(ctx context.Context, s Sale) (reference string, err error)
}

// Stripe is the stripe Method for the charge intent.
type Stripe struct {
	Seller Seller
	// Store records the tokens spent (section 8, step 4); it should be the Gate's.
	Store Store
}

// request is a stripe charge request as it is carried (sections 6.1 and 6.2). Recipient
// is read when present and never written: the profile names the seller.
type request struct {
	Amount        string   `json:"amount"`
	Currency      string   `json:"currency"`
	Decimals      *int     `json:"decimals,omitempty"`
	Description   string   `json:"description,omitempty"`
	ExternalID    string   `json:"externalId,omitempty"`
	Recipient     string   `json:"recipient,omitempty"`
	MethodDetails *details `json:"methodDetails,omitempty"`
}

type details struct {
	Metadata           map[string]string `json:"metadata,omitempty"`
	NetworkID          string            `json:"networkId"`
	PaymentMethodTypes []string          `json:"paymentMethodTypes"`
}

// Name implements Method.
func (Stripe) Name() string { return "stripe" }

// Propose implements Method. The request carries the quoted amount, the currency, and
// the seller's methodDetails. Currencies are three-letter lowercase ISO codes
// (section 6.1); another is not offered.
func (s Stripe) Propose(ctx context.Context, c Caller, q Quote) (*Proposal, error) {
	if !currencyShape.MatchString(q.Currency) {
		return nil, ErrSkip
	}
	p, err := s.Seller.Profile(ctx, c, q)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrSkip
	}
	if p.Network == "" || len(p.Types) == 0 {
		return nil, errors.New("stripe profile needs a network id and payment method types")
	}
	return &Proposal{Request: request{
		Amount: q.Amount, Currency: q.Currency, Decimals: p.Decimals, Description: q.Description, ExternalID: q.External,
		MethodDetails: &details{Metadata: p.Metadata, NetworkID: p.Network, PaymentMethodTypes: p.Types},
	}}, nil
}

// payload is the stripe credential payload (section 7).
type payload struct {
	SPT        string `json:"spt"`
	ExternalID string `json:"externalId,omitempty"`
}

func readPayload(body json.RawMessage) (payload, error) {
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return p, Refuse(Payload, "The credential payload does not match the stripe schema.")
	}
	if !tokenShape.MatchString(p.SPT) {
		return p, Refuse(Payload, "The credential payload needs a shared payment token, spt_...")
	}
	return p, nil
}

// Verify implements Method: the payload carries a shared payment token (section 7).
func (Stripe) Verify(_ context.Context, c Claim) error {
	_, err := readPayload(c.Payload)
	return err
}

// Settle implements Method, section 9: the token is taken so that it is spent once, and
// the Seller charges it under the idempotency key challengeID_token.
func (s Stripe) Settle(ctx context.Context, c Claim) (*Receipt, error) {
	pay, err := readPayload(c.Payload)
	if err != nil {
		return nil, err
	}
	var req request
	if err := json.Unmarshal(c.Request, &req); err != nil {
		return nil, fmt.Errorf("stripe: the request the gate issued does not decode: %w", err)
	}
	sum := sha256.Sum256([]byte(pay.SPT))
	took, err := s.Store.Take(ctx, "stripe/token/"+hex.EncodeToString(sum[:]), c.Until)
	if err != nil {
		return nil, err
	}
	if !took {
		return nil, Refuse(Failed, "The shared payment token has already been used.")
	}
	meta := map[string]string{}
	if req.MethodDetails != nil {
		for k, v := range req.MethodDetails.Metadata {
			meta[k] = v
		}
	}
	meta["challenge_id"] = c.Challenge.ID
	ref, err := s.Seller.Take(ctx, Sale{
		Caller: c.Caller, Token: pay.SPT, Amount: req.Amount, Currency: req.Currency, Description: req.Description,
		External: req.ExternalID, Challenge: c.Challenge.ID, Key: c.Challenge.ID + "_" + pay.SPT, Metadata: meta, Until: c.Until,
	})
	if err != nil {
		return nil, err
	}
	r := &Receipt{Method: "stripe", Reference: ref}
	if pay.ExternalID != "" {
		r.Fields = map[string]any{"externalId": pay.ExternalID}
	}
	return r, nil
}

// Limits are what a shared payment token is issued within (section 6.2): the challenge's
// amount and currency, valid until the challenge expires, to the seller's profile.
type Limits struct {
	Amount   string
	Currency string
	Expires  time.Time
	Network  string
	Types    []string
	// Description and External are the challenge's, for the payer to show or record.
	Description string
	External    string
	Metadata    map[string]string
}

// Wallet is a Payer for the stripe method, the client side of the same draft. Issue is
// the host's: creating a token takes a Stripe credential this package does not hold.
type Wallet struct {
	// Issue creates a shared payment token within the limits, spt_...
	Issue func(ctx context.Context, l Limits) (string, error)
	// Allow decides whether the payment may be made (section 11.6): it sees the
	// challenge and the limits, and returns an error to refuse. A nil Allow refuses every
	// payment, so that a wallet never pays an amount nobody approved.
	Allow func(c Challenge, l Limits) error
}

// Accepts implements Payer.
func (Wallet) Accepts() []Range { return []Range{{Method: "stripe", Intent: Charge, Weight: 1000}} }

// Pay implements Payer: it reads the request, asks Allow, issues the token and answers
// with the credential payload (section 7).
func (w Wallet) Pay(ctx context.Context, c Challenge, body json.RawMessage) (*Answer, error) {
	if w.Allow == nil || w.Issue == nil {
		return nil, errors.New("stripe wallet: Allow and Issue are required")
	}
	if c.Method != "stripe" || c.Intent != Charge {
		return nil, fmt.Errorf("stripe wallet: cannot answer %s/%s", c.Method, c.Intent)
	}
	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("stripe wallet: request: %w", err)
	}
	if !isAmount(req.Amount) || !currencyShape.MatchString(req.Currency) {
		return nil, errors.New("stripe wallet: request amount or currency is malformed")
	}
	if req.MethodDetails == nil || req.MethodDetails.NetworkID == "" || len(req.MethodDetails.PaymentMethodTypes) == 0 {
		return nil, errors.New("stripe wallet: request has no networkId and paymentMethodTypes")
	}
	if _, err := strconv.ParseInt(req.Amount, 10, 64); err != nil {
		return nil, errors.New("stripe wallet: request amount is out of range")
	}
	l := Limits{
		Amount: req.Amount, Currency: req.Currency, Network: req.MethodDetails.NetworkID, Types: req.MethodDetails.PaymentMethodTypes,
		Description: req.Description, External: req.ExternalID, Metadata: req.MethodDetails.Metadata,
	}
	if at, ok := c.Expiry(); ok {
		l.Expires = at
	}
	if err := w.Allow(c, l); err != nil {
		return nil, fmt.Errorf("stripe wallet: payment refused: %w", err)
	}
	spt, err := w.Issue(ctx, l)
	if err != nil {
		return nil, fmt.Errorf("stripe wallet: issue token: %w", err)
	}
	if !tokenShape.MatchString(spt) {
		return nil, errors.New("stripe wallet: the issued value is not a shared payment token")
	}
	return &Answer{Payload: payload{SPT: spt}}, nil
}
