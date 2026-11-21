// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"context"
	"errors"
)

// Caller is the party a request acts for. The host establishes it from the bearer token the
// agent presented; nothing in a request body or header names it.
type Caller struct {
	// Org is the merchant the agent buys from.
	Org string
	// Subject is the buyer the agent acts for.
	Subject string
	// Host is the host the request addressed; it names the brand whose catalog is sold.
	Host string
}

// State is where a checkout stands, in the seller's terms.
type State string

// States of a Checkout.
const (
	// Open can still be changed and completed.
	Open State = "open"
	// Paying has a payment started that waits on the buyer's bank or a deposit.
	Paying State = "paying"
	// Paid is complete: Receipt names the order.
	Paid State = "paid"
	// Failed had a completion refused: Refusal says why, and the checkout can be tried again.
	Failed State = "failed"
	// Canceled was abandoned.
	Canceled State = "canceled"
)

// Product is the one item a checkout sells.
type Product struct {
	// ID is the item id an agent names in line_items.
	ID   string
	Name string
}

// Receipt is the order a completed checkout created.
type Receipt struct {
	// ID is the seller's order id.
	ID string
}

// Refusal is why a completion was refused, in the spec's message vocabulary
// (MessageError.code and resolution) so the session can say it to the agent as it is.
type Refusal struct {
	Code       string
	Param      string
	Resolution string
	Message    string
}

// Checkout is a checkout as the seller's commerce system holds it.
type Checkout struct {
	ID    string
	State State
	Item  Product
	// Total is what completing charges, in minor units of the seller's currency, for the one
	// unit of Item the checkout sells.
	Total   int64
	Receipt *Receipt
	Refusal *Refusal
}

// Charge is the payment an agent submits on completion, after the service has matched it
// to one of the handlers the session declared.
type Charge struct {
	Handler    PaymentHandler
	Instrument string
	Credential string
	Token      string
}

// ErrNoItem is what a Backend returns for an item id it does not sell.
var ErrNoItem = errors.New("acp: no such item")

// Backend is what the seller's commerce system provides. Every method acts for a Caller.
// A Backend returns a *Fault for a refusal the agent can act on (Unsupported, Conflict,
// NotFound, Unavailable); any other error is an unexpected failure the service reports and
// answers as a processing_error without its text. Open and Change return ErrNoItem for an
// item the seller does not sell. Pay does not refuse for a payment the bank or token
// declined: it returns the Checkout in State Failed with a Refusal.
type Backend interface {
	// Currency is the lowercase ISO 4217 code the seller settles in.
	Currency() string
	// Open starts a checkout selling item.
	Open(ctx context.Context, who Caller, item string) (*Checkout, error)
	// Change replaces the item of a checkout that is still open.
	Change(ctx context.Context, who Caller, id, item string) (*Checkout, error)
	// Read answers where a checkout is.
	Read(ctx context.Context, who Caller, id string) (*Checkout, error)
	// Pay completes a checkout. key is the Idempotency-Key the completion arrived with: the
	// charge is bound to the checkout and the key, so completing twice is one purchase.
	Pay(ctx context.Context, who Caller, id, key string, pay Charge) (*Checkout, error)
	// Cancel abandons a checkout that is still open.
	Cancel(ctx context.Context, who Caller, id string) (*Checkout, error)
}
