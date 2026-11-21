// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Caller is who the host says is calling: the merchant organization whose store it is, the
// buyer the checkout belongs to, and the host name the request came to. The host derives it
// from its own authentication; this package never reads it from a request.
type Caller struct {
	Org     string
	Subject string
	Host    string
}

// Pick is a line a checkout is opened for: a store's item id and a quantity.
type Pick struct {
	ID       string
	Quantity int64
}

// Line is a line of a checkout as the store has it: the item, its title, its unit price in
// minor units from the store's catalog, and the quantity.
type Line struct {
	ID       string
	Title    string
	Price    int64
	Quantity int64
}

// Cart is a checkout as the store has it now.
type Cart struct {
	ID string
	// Currency is the ISO 4217 alphabetic code, upper case.
	Currency string
	Lines    []Line
	// Total is what the checkout charges, in minor units.
	Total    int64
	Canceled bool
}

// Payment is how a checkout is paid: the payment credential the credential provider
// released, and the instrument the Payment Mandate authorized it for.
type Payment struct {
	Token      string
	Instrument Instrument
}

// Paid is what the store answers a completed checkout with.
type Paid struct {
	// Order is the order the checkout became.
	Order string
	// Payment is the store's id for the payment.
	Payment string
	// PSP is the payment service provider's confirmation id.
	PSP string
	// Network is the payment network's confirmation id, "" when the store cannot know it.
	Network string
}

// Backend is what the merchant role needs from the store behind it. It does the pricing
// and the charging; this package does neither.
type Backend interface {
	// Open starts a checkout for items, priced from the store's catalog.
	Open(ctx context.Context, who Caller, items []Pick) (*Cart, error)
	// Read is a checkout as the store has it now, its lines priced from the catalog as it is
	// now. An unknown checkout, or one that is not the caller's, is ErrNotFound.
	Read(ctx context.Context, who Caller, id string) (*Cart, error)
	// Pay charges the checkout. It is idempotent in key: the same key is the same order.
	// A payment the store refuses is a *Declined.
	Pay(ctx context.Context, who Caller, id, key string, pay Payment) (*Paid, error)
	// Honours says whether the store can take payment by the instrument; when it cannot, the
	// error is an *Unsupported.
	Honours(ctx context.Context, who Caller, in Instrument) error
}

// Errors of a Backend.
var (
	// ErrNoItem: the store does not sell an item it was asked to open a checkout for.
	ErrNoItem = errors.New("ap2: the store does not sell that item")
	// ErrNotFound: no such checkout for this caller.
	ErrNotFound = errors.New("ap2: no such checkout")
	// ErrUnavailable: the store cannot answer now; nothing was decided.
	ErrUnavailable = errors.New("ap2: the store is unavailable")
)

// Unsupported is the refusal of a payment method the store cannot honour.
type Unsupported struct {
	Instrument Instrument
	Why        string
}

func (u *Unsupported) Error() string {
	return fmt.Sprintf("ap2: payment instrument %q of type %q is not supported: %s", u.Instrument.ID, u.Instrument.Type, u.Why)
}

// Declined is a payment the store refused: the credential was not usable or the issuer
// declined. Code is the store's code for it.
type Declined struct {
	Code    string
	Message string
}

func (d *Declined) Error() string {
	return fmt.Sprintf("ap2: payment declined (%s): %s", d.Code, d.Message)
}

// CodeOf is the AP2 error code a failure is answered with, and whether it is one to answer:
// a refusal is, and so is a payment the store declined; a store that could not answer is not.
func CodeOf(err error) (Code, bool) {
	var e *Error
	var u *Unsupported
	var d *Declined
	switch {
	case errors.As(err, &e):
		return e.Code, true
	case errors.As(err, &u), errors.Is(err, ErrNotFound), errors.Is(err, ErrNoItem):
		return InvalidMandate, true
	case errors.As(err, &d):
		return PaymentDeclined, true
	}
	return "", false
}

// Verdict is what a Ledger answers a claim with.
type Verdict int

// The verdicts of a Ledger.
const (
	// Fresh: the open mandate was free and is now claimed.
	Fresh Verdict = iota
	// Replay: the same closed mandate already holds the claim and was completed.
	Replay
	// Taken: another closed mandate was completed under the open mandate.
	Taken
	// Busy: a completion under the open mandate is in progress.
	Busy
)

// Ledger is the merchant's record of the open mandates it has completed a checkout under.
// An open mandate buys one checkout (checkout_mandate.md, "Line Items": it cannot be split
// across checkouts), and an agent that signs more than one closed mandate under it is
// refused at the second (security_and_privacy_considerations.md, "Double Spend").
type Ledger interface {
	// Begin claims the open mandate, named by its hash, for the closed mandate ref.
	Begin(ctx context.Context, open, ref string) (Verdict, Lease, error)
}

// Lease is a claim a Ledger granted as Fresh.
type Lease interface {
	// Keep records the checkout as completed: the open mandate is spent.
	Keep(ctx context.Context) error
	// Drop gives the claim up, the checkout not having been completed.
	Drop(ctx context.Context) error
}

// Memory is a Ledger held in the process. It serves one process; a merchant on more than one
// node needs a Ledger over shared storage.
type Memory struct {
	mu   sync.Mutex
	held map[string]*held
}

type held struct {
	ref  string
	kept bool
}

// Begin implements Ledger.
func (m *Memory) Begin(_ context.Context, open, ref string) (Verdict, Lease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.held == nil {
		m.held = map[string]*held{}
	}
	h, ok := m.held[open]
	switch {
	case !ok:
		m.held[open] = &held{ref: ref}
		return Fresh, &lease{m: m, open: open}, nil
	case !h.kept:
		return Busy, nil, nil
	case h.ref == ref:
		return Replay, nil, nil
	}
	return Taken, nil, nil
}

type lease struct {
	m    *Memory
	open string
}

func (l *lease) Keep(context.Context) error {
	l.m.mu.Lock()
	defer l.m.mu.Unlock()
	l.m.held[l.open].kept = true
	return nil
}

func (l *lease) Drop(context.Context) error {
	l.m.mu.Lock()
	defer l.m.mu.Unlock()
	delete(l.m.held, l.open)
	return nil
}
