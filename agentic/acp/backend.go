// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"context"
	"errors"
)

// Caller is the party a request acts for, established by the host from the bearer token.
type Caller struct {
	// Org is the merchant the agent buys from.
	Org string
	// Subject is the buyer the agent acts for.
	Subject string
	// Host is the host the request addressed.
	Host string
}

// State is where a checkout stands.
type State string

// States of a Checkout.
const (
	// Open can still be changed and completed.
	Open State = "open"
	// Paying has a payment started that is not yet settled.
	Paying State = "paying"
	// Paid is complete: Receipt names the order.
	Paid State = "paid"
	// Failed had a completion refused (Refusal); it can be completed again.
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

// Refusal is why a completion was refused, as a MessageError code and resolution.
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
	// Total is what completing charges for the one unit of Item, in minor units.
	Total   int64
	Receipt *Receipt
	Refusal *Refusal
}

// Charge is the payment an agent submits, matched to a handler the session declared.
type Charge struct {
	Handler    PaymentHandler
	Instrument string
	Credential string
	Token      string
}

// ErrNoItem is what a Backend returns for an item id it does not sell.
var ErrNoItem = errors.New("acp: no such item")

// Backend is the seller's commerce system. It returns a *Fault for a refusal the agent can act
// on and ErrNoItem for an unknown item; any other error is reported and answered as a
// processing_error. Pay returns a declined payment as a Checkout in State Failed.
type Backend interface {
	// Currency is the lowercase ISO 4217 code the seller settles in.
	Currency() string
	// Open starts a checkout selling item.
	Open(ctx context.Context, who Caller, item string) (*Checkout, error)
	// Change replaces the item of a checkout that is still open.
	Change(ctx context.Context, who Caller, id, item string) (*Checkout, error)
	// Read answers where a checkout is.
	Read(ctx context.Context, who Caller, id string) (*Checkout, error)
	// Pay completes a checkout; the charge is bound to the checkout and the Idempotency-Key.
	Pay(ctx context.Context, who Caller, id, key string, pay Charge) (*Checkout, error)
	// Cancel abandons a checkout that is still open.
	Cancel(ctx context.Context, who Caller, id string) (*Checkout, error)
}
