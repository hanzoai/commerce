// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"math/big"
	"strconv"
	"time"

	"github.com/hanzoai/commerce/payment/providers/circle"
)

// Submission is a signed payment as the caller presents it: which network it
// was signed for, the salt Terms issued with its nonce, the authorization and
// its signature.
type Submission struct {
	Network       string               `json:"network"`
	Salt          string               `json:"salt"`
	Authorization circle.Authorization `json:"authorization"`
	Signature     string               `json:"signature"`
}

// checked is a submission that passed verify, in its ONE canonical spelling:
// addresses, nonce, salt and signature lowercase, v as 27/28. Two submissions
// of the same payment canonicalise to the same value.
type checked struct {
	net   network
	auth  circle.Authorization
	salt  string
	sig   string
	cents int64

	after, before uint64 // the window, unix seconds
}

// window is the check that changes with the clock: the payment is valid now,
// and stays valid long enough for Gateway's batch (its minValiditySeconds) but
// no longer than MaxValidity, so an unanswered payment is followed for a
// bounded time.
func window(c checked, now time.Time) error {
	t := uint64(now.Unix())
	min, max := uint64(c.net.min/time.Second), uint64(MaxValidity/time.Second)
	switch {
	case c.after > t:
		return invalid(ReasonEarly, "validAfter %d is after now (%d)", c.after, t)
	case c.before <= t:
		return invalid(ReasonExpired, "validBefore %d has passed (now %d)", c.before, t)
	case c.before-t < min:
		return invalid(ReasonShort, "validBefore must be at least %d seconds from now; it is %d", min, c.before-t)
	case c.before-t > max:
		return invalid(ReasonLong, "validBefore must be at most %d seconds from now; it is %d", max, c.before-t)
	}
	return nil
}

// verify is everything a payment must satisfy that does not change with the
// clock, checked against Gateway's live network table, the treasury and the
// caller — cheapest first, so a signature is recovered only for a payment that
// could be accepted. The validity window is window's, and it gates only a
// payment presented for the FIRST time: one already recorded is answered with
// its record however much later it is presented again.
//
// Gateway checks the signature, the window and the recipient again at settle.
// Checking here as well is not redundancy for its own sake: it is what lets a
// refusal name its reason before any money is locked, and it is the only
// place the nonce binding and the limits are checked at all.
func verify(sub Submission, nets []network, treasury, org, payer string) (checked, error) {
	n, ok := lookup(nets, sub.Network)
	if !ok {
		return checked{}, invalid(ReasonNetwork, "network %q is not accepted; GET the terms for the networks that are", sub.Network)
	}
	p, err := sub.Authorization.Parse()
	if err != nil {
		return checked{}, invalid(ReasonPayload, "authorization: %v", err)
	}

	if circle.Hex(p.To[:]) != treasury {
		return checked{}, invalid(ReasonRecipient, "the payment is made out to %s; it must be made out to %s", circle.Hex(p.To[:]), treasury)
	}

	unit := centUnit(n.decimals)
	cents, rem := new(big.Int).QuoRem(p.Value, unit, new(big.Int))
	if rem.Sign() != 0 {
		return checked{}, invalid(ReasonCents, "value %s is not a whole number of cents (a multiple of %s base units)", p.Value, unit)
	}
	if cents.Cmp(big.NewInt(MinCents)) < 0 {
		return checked{}, invalid(ReasonLow, "value is %s cents; the minimum is %d", cents, MinCents)
	}
	if cents.Cmp(big.NewInt(MaxCents)) > 0 {
		return checked{}, invalid(ReasonHigh, "value is %s cents; the maximum is %d", cents, MaxCents)
	}

	salt, err := circle.Nonce(sub.Salt)
	if err != nil {
		return checked{}, invalid(ReasonUnbound, "salt: %v", err)
	}
	if bind(org, payer, salt) != p.Nonce {
		return checked{}, invalid(ReasonUnbound, "the nonce was not issued to this account; sign the nonce GET /v1/billing/crypto/gateway returns, and send its salt")
	}

	digest, err := n.domain.Digest(p)
	if err != nil {
		return checked{}, invalid(ReasonPayload, "domain: %v", err)
	}
	signer, err := circle.Recover(digest, sub.Signature)
	if err != nil {
		return checked{}, invalid(ReasonSignature, "signature: %v", err)
	}
	if signer != p.From {
		return checked{}, invalid(ReasonSignature, "the signature is not %s's over this payment on %s", circle.Hex(p.From[:]), n.id)
	}

	sig, err := circle.Canonical(sub.Signature)
	if err != nil {
		return checked{}, invalid(ReasonSignature, "signature: %v", err)
	}

	return checked{
		net:    n,
		after:  p.ValidAfter,
		before: p.ValidBefore,
		auth: circle.Authorization{
			From:        circle.Hex(p.From[:]),
			To:          circle.Hex(p.To[:]),
			Value:       p.Value.String(),
			ValidAfter:  strconv.FormatUint(p.ValidAfter, 10),
			ValidBefore: strconv.FormatUint(p.ValidBefore, 10),
			Nonce:       circle.Hex(p.Nonce[:]),
		},
		salt:  circle.Hex(salt[:]),
		sig:   sig,
		cents: cents.Int64(),
	}, nil
}
