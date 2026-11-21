// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"errors"
	"fmt"
)

// Code is an AP2 error code, the vocabulary of a receipt's `error` claim for an action
// authorization (agent_authorization.md, "Errors").
type Code string

const (
	// InvalidCredential: the mandate fails verification (signature, key, hash binding,
	// audience, nonce, validity period, structure). A terminal error.
	InvalidCredential Code = "invalid_credential"
	// UnresolvedConstraint: the mandate holds a constraint the verifier does not know, or the
	// verifier cannot show the closed mandate conforms to its constraints. The agent may fall
	// back to a directly approved closed mandate.
	UnresolvedConstraint Code = "unresolved_constraint"
	// InvalidMandate: the mandate is authentic but does not approve the requested action. A
	// terminal error.
	InvalidMandate Code = "invalid_mandate"
	// MandatesNotSupported: the verifier does not support mandates for this action.
	MandatesNotSupported Code = "mandates_not_supported"
	// PaymentDeclined is the code of a receipt for a payment the processor refused after the
	// mandates were accepted. The receipt schemas take any unique string as an error code;
	// this one extends the four above.
	PaymentDeclined Code = "payment_declined"
)

// The reasons a mandate is refused. Each is wrapped by an *Error, so errors.Is names the
// check that failed.
var (
	ErrMalformed  = errors.New("malformed")
	ErrAlgorithm  = errors.New("unsupported algorithm")
	ErrKey        = errors.New("no usable key")
	ErrSignature  = errors.New("bad signature")
	ErrDisclosure = errors.New("bad disclosure")
	ErrChain      = errors.New("bad delegation chain")
	ErrBinding    = errors.New("hash binding does not hold")
	ErrAudience   = errors.New("wrong audience")
	ErrNonce      = errors.New("wrong nonce")
	ErrExpired    = errors.New("expired")
	ErrNotYet     = errors.New("not yet valid")
	ErrType       = errors.New("wrong mandate type")
	ErrMandate    = errors.New("bad mandate")
	ErrCheckout   = errors.New("bad checkout")
	ErrHash       = errors.New("checkout hash does not match the checkout")
	ErrConstraint = errors.New("constraint not met")
	ErrStale      = errors.New("checkout no longer matches the store")
	ErrPayment    = errors.New("payment does not match the checkout")
	ErrSpent      = errors.New("open mandate already used")
	ErrBusy       = errors.New("open mandate in use")
)

// Error is the refusal of a mandate, with the AP2 code a receipt carries.
type Error struct {
	Code Code
	Err  error
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// refuse states a refusal: reason is one of the Err values, detail says what.
func refuse(code Code, reason error, format string, args ...any) *Error {
	return &Error{Code: code, Err: fmt.Errorf("%w: %s", reason, fmt.Sprintf(format, args...))}
}

func credential(reason error, format string, args ...any) *Error {
	return refuse(InvalidCredential, reason, format, args...)
}

func mandate(reason error, format string, args ...any) *Error {
	return refuse(InvalidMandate, reason, format, args...)
}

func unresolved(format string, args ...any) *Error {
	return refuse(UnresolvedConstraint, ErrConstraint, format, args...)
}
