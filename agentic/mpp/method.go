// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Caller is who a request is made by and for. The host's function returns it; the Gate
// never reads it from a header.
type Caller struct {
	// Org is the merchant organization the resource belongs to.
	Org string
	// Subject is the buyer; empty when the resource asks for no other authentication
	// (section 11.7).
	Subject string
}

// Quote is what the host asks a request to pay: the amount in base units as ASCII digits,
// and the currency (draft-payment-intent-charge-00 section 5.1).
type Quote struct {
	Amount      string
	Currency    string
	Description string
	// External is the merchant's reference (externalId). A credential is accepted only
	// for the reference it was issued for.
	External string
}

// Valid checks that the quote is a positive amount and a currency.
func (q Quote) Valid() error {
	if !isAmount(q.Amount) || q.Amount == "0" {
		return fmt.Errorf("quote amount %q is not a positive integer in base units", q.Amount)
	}
	if q.Currency == "" {
		return errors.New("quote has no currency")
	}
	return nil
}

// isAmount is the amount form: ASCII digits, no sign, no leading zero.
func isAmount(s string) bool {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// cmpAmount orders two amounts that isAmount accepts.
func cmpAmount(a, b string) int {
	switch {
	case len(a) != len(b):
		if len(a) < len(b) {
			return -1
		}
		return 1
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// terms is the request every method of the charge intent shares
// (draft-payment-intent-charge-00 section 5.1).
type terms struct {
	Amount        string          `json:"amount"`
	Currency      string          `json:"currency"`
	Recipient     string          `json:"recipient,omitempty"`
	Description   string          `json:"description,omitempty"`
	ExternalID    string          `json:"externalId,omitempty"`
	MethodDetails json.RawMessage `json:"methodDetails,omitempty"`
}

// termsOf decodes the wire form of a request into its JSON and its shared terms.
func termsOf(wire string) ([]byte, terms, error) {
	var t terms
	b, err := raw.DecodeString(wire)
	if err != nil {
		return nil, t, err
	}
	return b, t, json.Unmarshal(b, &t)
}

// Errors a host's Caller and Price, and a Method's Propose, return.
var (
	// ErrSkip is a Method not offering the quote.
	ErrSkip = errors.New("mpp: method does not offer this quote")
	// ErrUnauthorized is the host saying the request lacks the authentication the
	// resource needs: 401, with no Payment challenge (section 4.4.3).
	ErrUnauthorized = errors.New("mpp: request is not authenticated")
	// ErrDenied is the host saying policy forbids the request: 403, no challenge, nothing
	// settled (section 4.2).
	ErrDenied = errors.New("mpp: request is denied")
)

// Proposal is what a Method puts in a challenge for a quote.
type Proposal struct {
	// Request is the method's request object.
	Request any
}

// Claim is a credential the Gate has checked and hands to a Method: the echoed challenge
// is the one issued, unexpired, for the quoted amount.
type Claim struct {
	Caller    Caller
	Challenge Challenge
	// Request is the challenge's decoded request.
	Request json.RawMessage
	Source  string
	Payload json.RawMessage
	// Until is when a method may forget what it recorded for the challenge.
	Until time.Time
}

// Method is a payment method (section 6). Verify and Settle return a *Problem to say the
// payment was refused and nothing charged; any other error is an outcome not known, and
// the Gate keeps the credential spent.
type Method interface {
	// Name is the method identifier: lowercase ASCII letters (section 6.1).
	Name() string
	// Propose makes the request object for the quote, or returns ErrSkip.
	Propose(ctx context.Context, c Caller, q Quote) (*Proposal, error)
	// Verify checks the payload against the request without moving money.
	Verify(ctx context.Context, c Claim) error
	// Settle takes the payment, once. The returned Receipt need only name the reference.
	Settle(ctx context.Context, c Claim) (*Receipt, error)
}
