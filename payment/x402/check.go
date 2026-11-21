// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// The error reasons of spec section 9. They are the vocabulary a client matches
// on, so each is spelled once, here.
const (
	ReasonInsufficientFunds   = "insufficient_funds"
	ReasonValidAfter          = "invalid_exact_evm_payload_authorization_valid_after"
	ReasonValidBefore         = "invalid_exact_evm_payload_authorization_valid_before"
	ReasonValueMismatch       = "invalid_exact_evm_payload_authorization_value_mismatch"
	ReasonSignature           = "invalid_exact_evm_payload_signature"
	ReasonRecipientMismatch   = "invalid_exact_evm_payload_recipient_mismatch"
	ReasonInvalidNetwork      = "invalid_network"
	ReasonInvalidPayload      = "invalid_payload"
	ReasonInvalidRequirements = "invalid_payment_requirements"
	ReasonInvalidScheme       = "invalid_scheme"
	ReasonInvalidVersion      = "invalid_x402_version"
	ReasonInvalidTransaction  = "invalid_transaction_state"
	ReasonVerifyError         = "unexpected_verify_error"
	ReasonSettleError         = "unexpected_settle_error"
)

// Error is a refusal that carries the specification's own reason, which is what
// the wire reports. Message is free-form diagnosis for logs; a client must match
// Reason and never Message (spec section 9).
type Error struct {
	Reason  string
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return e.Reason
	}
	return e.Reason + ": " + e.Message
}

func refuse(reason, format string, args ...any) *Error {
	return &Error{Reason: reason, Message: fmt.Sprintf(format, args...)}
}

// ReasonOf reports the spec reason an error carries, and fallback for any error
// that is not an *Error.
func ReasonOf(err error, fallback string) string {
	if e, ok := errors.AsType[*Error](err); ok {
		return e.Reason
	}
	return fallback
}

// Verify checks everything about pay that is true or false for all time: that it
// speaks this protocol version, that the terms it echoes are the terms offered,
// that it pays exactly the offered amount to exactly the offered payee, and that
// the signature recovers to the payer it names.
//
// It does not read the clock: that is InWindow, kept apart because the two have
// different lifetimes. A bad signature is bad forever; an expired authorization
// was valid when it was accepted. Verify does not know the payer's balance or
// whether the nonce is spent; those are the facilitator's to check (spec 7.1).
func Verify(offered PaymentRequirements, pay PaymentPayload) error {
	if pay.X402Version != Version {
		return refuse(ReasonInvalidVersion, "x402Version %d, want %d", pay.X402Version, Version)
	}
	if err := Same(offered, pay.Accepted); err != nil {
		return err
	}
	if offered.Scheme != SchemeExact {
		return refuse(ReasonInvalidScheme, "scheme %q; only %q is implemented", offered.Scheme, SchemeExact)
	}
	extra := extraOf(offered)
	if extra.AssetTransferMethod != TransferEIP3009 {
		return refuse(ReasonInvalidScheme, "assetTransferMethod %q; only %q is implemented", extra.AssetTransferMethod, TransferEIP3009)
	}
	chain, err := ChainID(offered.Network)
	if err != nil {
		return err
	}
	auth, err := pay.Payload.Authorization.Parse()
	if err != nil {
		return refuse(ReasonInvalidPayload, "authorization: %v", err)
	}
	to, err := Address(offered.PayTo)
	if err != nil {
		return refuse(ReasonInvalidRequirements, "payTo: %v", err)
	}
	if auth.To != to {
		return refuse(ReasonRecipientMismatch, "authorized payee %s, required %s", pay.Payload.Authorization.To, offered.PayTo)
	}
	want, err := amount(offered.Amount)
	if err != nil {
		return refuse(ReasonInvalidRequirements, "amount: %v", err)
	}
	if auth.Value.Cmp(want) != 0 {
		return refuse(ReasonValueMismatch, "authorized %s, required %s", pay.Payload.Authorization.Value, offered.Amount)
	}
	digest, err := Domain{Name: extra.Name, Version: extra.Version, ChainID: chain, VerifyingContract: offered.Asset}.Digest(auth)
	if err != nil {
		return refuse(ReasonInvalidRequirements, "%v", err)
	}
	signer, err := Recover(digest, pay.Payload.Signature)
	if err != nil {
		return refuse(ReasonSignature, "%v", err)
	}
	if signer != auth.From {
		return refuse(ReasonSignature, "recovered %s, claimed %s", Hex(signer[:]), pay.Payload.Authorization.From)
	}
	return nil
}

// InWindow reports whether an authorization may be accepted at now: validAfter
// is exclusive and validBefore is exclusive, as the token contract checks them
// (EIP-3009: block.timestamp > validAfter and < validBefore).
func InWindow(a Authorization, now time.Time) error {
	p, err := a.Parse()
	if err != nil {
		return refuse(ReasonInvalidPayload, "authorization: %v", err)
	}
	t := uint64(now.Unix())
	if t <= p.ValidAfter {
		return refuse(ReasonValidAfter, "authorization is not valid until after %d (now %d)", p.ValidAfter, t)
	}
	if t >= p.ValidBefore {
		return refuse(ReasonValidBefore, "authorization expired at %d (now %d)", p.ValidBefore, t)
	}
	return nil
}

// Same reports whether the requirements a client echoed are the ones offered,
// field for field, including the EIP-712 domain in Extra: a payload that could
// name its own token name, version, asset or chain would sign a message of its
// own devising and verify perfectly against itself (spec 6.1.2, parameter
// matching).
func Same(offered, echoed PaymentRequirements) error {
	switch {
	case echoed.Scheme != offered.Scheme:
		return refuse(ReasonInvalidScheme, "scheme %q, offered %q", echoed.Scheme, offered.Scheme)
	case echoed.Network != offered.Network:
		return refuse(ReasonInvalidNetwork, "network %q, offered %q", echoed.Network, offered.Network)
	case strings.TrimSpace(echoed.Amount) != strings.TrimSpace(offered.Amount):
		return refuse(ReasonValueMismatch, "amount %q, offered %q", echoed.Amount, offered.Amount)
	case !sameAddress(echoed.Asset, offered.Asset):
		return refuse(ReasonInvalidRequirements, "asset %q, offered %q", echoed.Asset, offered.Asset)
	case !sameAddress(echoed.PayTo, offered.PayTo):
		return refuse(ReasonRecipientMismatch, "payTo %q, offered %q", echoed.PayTo, offered.PayTo)
	case extraOf(echoed) != extraOf(offered):
		return refuse(ReasonInvalidRequirements, "extra %+v, offered %+v", extraOf(echoed), extraOf(offered))
	}
	return nil
}

// extraOf is the requirement's EIP-712 domain with the spec's default filled in:
// an absent assetTransferMethod means eip3009, so a client that omits it and a
// server that states it agree.
func extraOf(r PaymentRequirements) Extra {
	var e Extra
	if r.Extra != nil {
		e = *r.Extra
	}
	if e.AssetTransferMethod == "" {
		e.AssetTransferMethod = TransferEIP3009
	}
	return e
}

func sameAddress(a, b string) bool {
	x, errx := Address(a)
	y, erry := Address(b)
	return errx == nil && erry == nil && x == y
}
