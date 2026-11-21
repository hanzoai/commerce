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

// Caller is who a request is made by and for, as the host knows them. The Gate never
// reads it from a header: the host's function returns it.
type Caller struct {
	// Org is the merchant organization the resource belongs to.
	Org string
	// Subject is the buyer. It is empty when the resource asks for no other
	// authentication, which the draft's privacy rule favours (section 11.7).
	Subject string
}

// Quote is what the host asks a request to pay: the amount in the smallest unit of the
// currency, as ASCII digits, and the currency in the form the methods share (an ISO 4217
// code in lower case for fiat). The Gate never chooses either.
type Quote struct {
	Amount      string
	Currency    string
	Description string
	// External is the merchant's reference for the sale (the externalId request member).
	// A credential is accepted only for the external reference it was issued for.
	External string
}

// Valid reports whether the quote is one a challenge can carry: a positive amount of
// digits without leading zeros, and a currency.
func (q Quote) Valid() error {
	if !isAmount(q.Amount) || q.Amount == "0" {
		return fmt.Errorf("quote amount %q is not a positive integer in base units", q.Amount)
	}
	if q.Currency == "" {
		return errors.New("quote has no currency")
	}
	return nil
}

// isAmount is the amount form of the draft: ASCII digits, no sign, no leading zero.
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

// Terms is the request every method of the charge intent shares
// (draft-payment-intent-charge-00 section 5.1). MethodDetails is the method's own.
type Terms struct {
	Amount        string          `json:"amount"`
	Currency      string          `json:"currency"`
	Recipient     string          `json:"recipient,omitempty"`
	Description   string          `json:"description,omitempty"`
	ExternalID    string          `json:"externalId,omitempty"`
	MethodDetails json.RawMessage `json:"methodDetails,omitempty"`
}

// Sentinels a host's Caller and Price functions, and a Method's Propose, return.
var (
	// ErrSkip is a Method saying it cannot take this quote, so no challenge is made for it.
	ErrSkip = errors.New("mpp: method does not offer this quote")
	// ErrUnauthorized is the host saying the request carries no authentication the
	// resource needs. The Gate answers 401 with no Payment challenge; the other scheme's
	// challenge is the host's, so authenticate before the Gate (section 4.4.3).
	ErrUnauthorized = errors.New("mpp: request is not authenticated")
	// ErrDenied is the host saying policy forbids the request whatever it pays. The Gate
	// answers 403 with no challenge and settles nothing (section 4.2).
	ErrDenied = errors.New("mpp: request is denied")
)

// Proposal is what a Method puts in a challenge for a quote.
type Proposal struct {
	// Request is the method's request object, serialized with JCS into the request
	// parameter.
	Request any
	// Opaque is server-defined correlation data, a flat map of strings, returned by the
	// client unchanged and covered by the binding (section 5.1.2).
	Opaque map[string]string
}

// Claim is a credential that has passed the Gate's checks and is handed to a Method: the
// challenge it echoes is the one the Gate issued (the binding holds, it has not expired,
// the amount is the quote's), and Request is that challenge's decoded request.
type Claim struct {
	Caller    Caller
	Challenge Challenge
	Request   json.RawMessage
	Source    string
	Payload   json.RawMessage
	// Key is the Idempotency-Key of the request, empty when it carries none.
	Key string
	At  time.Time
	// Until is the instant a method may forget what it recorded for this challenge: the
	// Gate keeps its own state for the challenge until then.
	Until time.Time
}

// Method is a payment method (section 6): the plug-in that makes the request a
// challenge carries, checks a credential's proof, and settles it.
//
// Settle and Verify return a *Problem to say the payment was refused and that nothing
// was charged. Any other error means the outcome is unknown, and the Gate keeps the
// credential spent and answers 500, so that a payment that may have settled is never
// attempted twice.
type Method interface {
	// Name is the method identifier: lowercase ASCII letters (section 6.1).
	Name() string
	// Propose makes the request object for the quote. It returns ErrSkip when the
	// method cannot take this quote or caller.
	Propose(ctx context.Context, c Caller, q Quote) (*Proposal, error)
	// Verify checks the payload against the request without moving money. The Gate calls
	// it before Settle; a Problem frees the credential to be sent again.
	Verify(ctx context.Context, c Claim) error
	// Settle takes the payment, once. The Gate guarantees it is not called twice for a
	// challenge. The Receipt it returns need only name the method and the reference;
	// the Gate fills in the status and the settlement time.
	Settle(ctx context.Context, c Claim) (*Receipt, error)
}
