// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Caller is who the host says is calling: the merchant organization, the buyer the checkout
// belongs to, and the host name the request came to. The host derives it from its own
// authentication.
type Caller struct {
	Org     string
	Subject string
	Host    string
}

// Line is a line of a checkout as the store has it: the item, its title, its unit price in minor
// units from the store's catalog, and the quantity.
type Line struct {
	ID       string
	Title    string
	Price    int64
	Quantity int64
}

// Cart is a checkout as the store has it now. Currency is the ISO 4217 code, upper case; Total is
// what it charges, in minor units; Canceled, Pending and Paid say where it is: canceled, taken for
// a payment begun and not ended, or paid.
type Cart struct {
	ID       string
	Currency string
	Lines    []Line
	Total    int64
	Canceled bool
	Pending  bool
	Paid     bool
}

// Payment is how a checkout is paid: the credential the credential provider released, the
// instrument the Payment Mandate authorized, and the checkout the mandates were verified against.
type Payment struct {
	Token      string
	Instrument Instrument
	Cart       Cart
}

// Paid is what the store answers a completed checkout with. Network is the payment network's
// confirmation id, empty when the store cannot know it.
type Paid struct {
	Order   string
	Payment string
	PSP     string
	Network string
}

// Backend is what the merchant role needs from the store behind it, which does the pricing and the
// charging.
type Backend interface {
	// Read is a checkout as the store has it now, its lines priced from the catalog as it is now.
	// An unknown checkout, or one that is not the caller's, is ErrNotFound.
	Read(ctx context.Context, who Caller, id string) (*Cart, error)
	// Pay charges the checkout, which must still be pay.Cart: its buyer can change it between the
	// verification and the charge, so the store checks where it takes the checkout for payment,
	// and otherwise charges nothing and answers ErrStale. It is idempotent in key. A payment the
	// store refuses is a *Declined.
	Pay(ctx context.Context, who Caller, id, key string, pay Payment) (*Paid, error)
	// Honours says whether the store takes payment by the instrument, else an *Unsupported. pinned
	// says the mandate limits which instrument may pay; a store that cannot tie its credential to
	// an instrument refuses that rather than ignore the limit.
	Honours(ctx context.Context, who Caller, in Instrument, pinned bool) error
}

// Errors of a Backend.
var (
	ErrNoItem      = errors.New("ap2: the store does not sell that item")
	ErrNotFound    = errors.New("ap2: no such checkout")
	ErrUnavailable = errors.New("ap2: the store is unavailable") // nothing was decided
)

// Unsupported is the refusal of what the store cannot honour: a payment instrument, or an item it
// does not sell through mandates.
type Unsupported struct {
	Instrument Instrument
	Item       string
	Why        string
}

func (u *Unsupported) Error() string {
	if u.Item != "" {
		return fmt.Sprintf("ap2: item %q is not sold through mandates: %s", u.Item, u.Why)
	}
	return fmt.Sprintf("ap2: payment instrument %q of type %q is not supported: %s", u.Instrument.ID, u.Instrument.Type, u.Why)
}

// Declined is a payment the store refused; Code is the store's code for it.
type Declined struct {
	Code    string
	Message string
}

func (d *Declined) Error() string {
	return fmt.Sprintf("ap2: payment declined (%s): %s", d.Code, d.Message)
}

// CodeOf is the AP2 error code a failure is answered with, and whether it is one to answer: a
// refusal and a declined payment are; a store that could not answer is not.
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
	Fresh  Verdict = iota // the open mandate was free and is now claimed
	Replay                // the same closed mandate holds the claim, completed
	Taken                 // another closed mandate was completed under the open mandate
	Busy                  // a completion under the open mandate was begun and has not ended
)

// Claim is who holds an open mandate: the closed mandate that claimed it, the checkout it was
// completing, and when.
type Claim struct {
	Ref      string
	Checkout string
	At       time.Time
}

// Ledger is the merchant's record of the open mandates it has completed a checkout under. An open
// mandate buys one checkout (checkout_mandate.md, "Line Items") and a second closed mandate under
// it is refused (security_and_privacy_considerations.md, "Double Spend"). A claim begun and not
// settled stays: a charge of unknown outcome may have moved money, so no clock releases it.
type Ledger interface {
	// Begin claims the open mandate, named by its id, for closed mandate ref completing checkout,
	// and answers with the claim that holds it: this one when Fresh.
	Begin(ctx context.Context, open, ref, checkout string) (Verdict, Claim, error)
	// Settle ends the claim ref holds: kept records the checkout completed; otherwise the claim
	// is given up, nothing having been charged. A claim another closed mandate holds is left alone.
	Settle(ctx context.Context, open, ref string, kept bool) error
}
