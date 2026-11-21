// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"context"
	"errors"
)

// Caller is who a request acts for, as the host decides it. The host derives it
// from its own authentication, never from a request header taken on trust.
type Caller struct {
	// Org is the merchant organization that sells.
	Org string
	// Subject is the buyer the checkout is for.
	Subject string
	// Host is the brand host the request addressed, for a backend that prices
	// by brand.
	Host string
}

// key scopes idempotency and ownership to one buyer of one merchant.
func (c Caller) key() string { return c.Org + "\x00" + c.Subject }

// Sentinel errors a Backend or Identify function returns.
var (
	// ErrNotFound means the resource does not exist or is not the caller's.
	ErrNotFound = errors.New("not found")
	// ErrUnauthorized means the caller is not authenticated (HTTP 401).
	ErrUnauthorized = errors.New("unauthorized")
	// ErrForbidden means the caller is authenticated but may not do this (HTTP 403).
	ErrForbidden = errors.New("forbidden")
)

// Refusal is a business outcome that stops an operation. It is answered with
// HTTP 200: with Current set, the response is that checkout carrying Messages;
// without it, the response is an error response with status "error"
// (shopping/checkout, "Error Handling").
type Refusal struct {
	Messages    []Message
	ContinueURL string
	Current     *Checkout
}

func (r *Refusal) Error() string {
	if len(r.Messages) > 0 {
		return r.Messages[0].Code + ": " + r.Messages[0].Content
	}
	return "refused"
}

// Unavailable means the backend cannot serve the request now (HTTP 503).
type Unavailable struct {
	// RetryAfter is the seconds after which to retry, 0 for none.
	RetryAfter int
}

func (u *Unavailable) Error() string { return "temporarily unavailable" }

// Backend is what the Checkout capability needs from the merchant. It returns a
// checkout resource without the ucp envelope, which the server adds; the
// resource must satisfy Checkout.Check once it has one.
//
// A backend enforces the checkout lifecycle atomically: the server reads the
// checkout before an update, completion or cancellation to answer the cases
// the specification fixes, but only the backend can make the change safe against
// a concurrent one.
type Backend interface {
	// Create opens a checkout for the items of the request.
	Create(ctx context.Context, who Caller, in *CreateRequest) (*Checkout, error)
	// Read returns a checkout; ErrNotFound when it is not the caller's.
	Read(ctx context.Context, who Caller, id string) (*Checkout, error)
	// Update replaces the state of a checkout that is not completed or canceled.
	Update(ctx context.Context, who Caller, id string, in *UpdateRequest) (*Checkout, error)
	// Complete places the order. key is the request's Idempotency-Key, which
	// binds the payment to this checkout so completing twice is one purchase.
	Complete(ctx context.Context, who Caller, id, key string, in *CompleteRequest) (*Checkout, error)
	// Cancel cancels a checkout that is not completed or canceled.
	Cancel(ctx context.Context, who Caller, id string) (*Checkout, error)
}

// Orders is implemented by a backend that serves the Order capability.
type Orders interface {
	// Order returns the current-state snapshot of an order the caller may see;
	// ErrNotFound when there is none.
	Order(ctx context.Context, who Caller, id string) (*Order, error)
}
