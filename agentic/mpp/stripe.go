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

// The stripe method of the charge intent (draft-stripe-charge-00): the payer issues a
// Shared Payment Token for the challenged amount and the server creates and confirms a
// Stripe PaymentIntent on it.

var (
	currencyShape = regexp.MustCompile(`^[a-z]{3}$`)          // section 6.1
	tokenShape    = regexp.MustCompile(`^spt_[A-Za-z0-9_]+$`) // section 7
)

// Profile is the seller's methodDetails (section 6.2): the Stripe Business Network profile
// the token is issued to, the payment method types the seller processes, and metadata.
type Profile struct {
	Network  string
	Types    []string
	Metadata map[string]string
}

// Sale is one shared payment token to charge (section 9).
type Sale struct {
	Caller Caller
	Token  string
	// Amount and Currency are the challenge's.
	Amount      string
	Currency    string
	Description string
	External    string
	// Challenge is the challenge id, carried to Stripe as metadata.
	Challenge string
	// Key is the idempotency key: the challenge id and the token.
	Key string
	// Metadata is methodDetails.metadata with challenge_id added.
	Metadata map[string]string
	// Until is when the Seller may forget what it recorded for the challenge.
	Until time.Time
}

// Seller is the merchant side of the stripe method.
type Seller interface {
	// Profile is the seller's methodDetails for the quote; nil or ErrSkip offers none.
	Profile(ctx context.Context, c Caller, q Quote) (*Profile, error)
	// Take creates and confirms the PaymentIntent on the token and returns its reference
	// once it has succeeded (section 9). A refusal is a *Problem: Failed for a token or
	// card that did not pay, Action when the payer must act further, Insufficient when the
	// token cannot cover the amount.
	Take(ctx context.Context, s Sale) (reference string, err error)
}

// Stripe is the stripe Method.
type Stripe struct {
	Seller Seller
	// Store records the tokens spent (section 8 step 4); it is the Gate's.
	Store Store
}

// request is the stripe request (sections 6.1, 6.2).
type request struct {
	Amount        string   `json:"amount"`
	Currency      string   `json:"currency"`
	Description   string   `json:"description,omitempty"`
	ExternalID    string   `json:"externalId,omitempty"`
	MethodDetails *details `json:"methodDetails,omitempty"`
}

type details struct {
	Metadata           map[string]string `json:"metadata,omitempty"`
	NetworkID          string            `json:"networkId"`
	PaymentMethodTypes []string          `json:"paymentMethodTypes"`
}

// Name implements Method.
func (Stripe) Name() string { return "stripe" }

// Propose implements Method. Currencies are lowercase three-letter ISO codes; another is
// not offered.
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
		Amount: q.Amount, Currency: q.Currency, Description: q.Description, ExternalID: q.External,
		MethodDetails: &details{Metadata: p.Metadata, NetworkID: p.Network, PaymentMethodTypes: p.Types},
	}}, nil
}

// payload is the credential payload (section 7).
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

// Verify implements Method: the payload carries a shared payment token.
func (Stripe) Verify(_ context.Context, c Claim) error {
	_, err := readPayload(c.Payload)
	return err
}

// Settle implements Method (section 9): the token is taken, so it is spent once (section
// 8 step 4), and the Seller charges it under the key challengeID_token.
func (s Stripe) Settle(ctx context.Context, c Claim) (*Receipt, error) {
	pay, err := readPayload(c.Payload)
	if err != nil {
		return nil, err
	}
	var req request
	if err := json.Unmarshal(c.Request, &req); err != nil {
		return nil, fmt.Errorf("stripe: the issued request does not decode: %w", err)
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

// Limits are what a shared payment token is issued within (section 6.2).
type Limits struct {
	Amount   string
	Currency string
	Expires  time.Time
	Network  string
}

// Wallet is the Payer for the stripe method. Issue creates the token, which takes a
// Stripe credential this package does not hold.
type Wallet struct {
	// Issue creates a shared payment token within the limits.
	Issue func(ctx context.Context, l Limits) (string, error)
	// Allow decides whether the payment may be made (section 11.6), returning an error to
	// refuse. Nil refuses every payment.
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
	if _, err := strconv.ParseUint(req.Amount, 10, 63); err != nil || !isAmount(req.Amount) || !currencyShape.MatchString(req.Currency) {
		return nil, errors.New("stripe wallet: request amount or currency is malformed")
	}
	if req.MethodDetails == nil || req.MethodDetails.NetworkID == "" || len(req.MethodDetails.PaymentMethodTypes) == 0 {
		return nil, errors.New("stripe wallet: request has no networkId and paymentMethodTypes")
	}
	l := Limits{Amount: req.Amount, Currency: req.Currency, Network: req.MethodDetails.NetworkID}
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
