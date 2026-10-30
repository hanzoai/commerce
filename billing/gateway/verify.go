// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"math/big"
	"strconv"
	"time"

	"github.com/hanzoai/commerce/payment/eip3009"
)

// Submission is a signed payment as the caller presents it: which network it
// was signed for, the salt Terms issued with its nonce, the authorization and
// its signature.
type Submission struct {
	Network       string                `json:"network"`
	Salt          string                `json:"salt"`
	Authorization eip3009.Authorization `json:"authorization"`
	Signature     string                `json:"signature"`
}

// checked is a submission that passed verify, in its ONE canonical spelling:
// addresses, nonce, salt and signature lowercase, v as 27/28. Two submissions
// of the same payment canonicalise to the same value.
type checked struct {
	chain Chain
	auth  eip3009.Authorization
	salt  string
	sig   string
	cents int64

	after, before uint64 // the window, unix seconds
}

// window is the check that changes with the clock: the payment is valid now,
// stays valid long enough for the relayer's transaction to land (MinValidity)
// and no longer than MaxValidity, and was not made valid more than MaxAge ago,
// which bounds how far back the chain is searched for its use.
func window(c checked, now time.Time) error {
	t := uint64(now.Unix())
	min, max := uint64(MinValidity/time.Second), uint64(MaxValidity/time.Second)
	switch {
	case c.after > t:
		return invalid(ReasonEarly, "validAfter %d is after now (%d)", c.after, t)
	case t-c.after > uint64(MaxAge/time.Second):
		return invalid(ReasonStale, "validAfter must be at most %d seconds ago; it is %d", uint64(MaxAge/time.Second), t-c.after)
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
// clock, checked against the network table, the treasury and the caller —
// cheapest first, so a signature is recovered only for a payment that could be
// accepted. The validity window is window's, and it gates only a payment
// presented for the FIRST time: one already recorded is answered with its
// record however much later it is presented again.
//
// The USDC contract checks the signature, the window and the nonce again when
// the authorization is used. Checking here as well is what lets a refusal name
// its reason before any gas is spent, and it is the only place the recipient,
// the nonce binding and the limits are checked at all.
func verify(sub Submission, chains []Chain, treasury, org, payer string) (checked, error) {
	c, ok := lookup(chains, sub.Network)
	if !ok {
		return checked{}, invalid(ReasonNetwork, "network %q is not accepted; GET the terms for the networks that are", sub.Network)
	}
	p, err := sub.Authorization.Parse()
	if err != nil {
		return checked{}, invalid(ReasonPayload, "authorization: %v", err)
	}

	if eip3009.Hex(p.To[:]) != treasury {
		return checked{}, invalid(ReasonRecipient, "the payment is made out to %s; it must be made out to %s", eip3009.Hex(p.To[:]), treasury)
	}

	unit := centUnit(usdcDecimals)
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

	salt, err := eip3009.Nonce(sub.Salt)
	if err != nil {
		return checked{}, invalid(ReasonUnbound, "salt: %v", err)
	}
	if bind(org, payer, salt) != p.Nonce {
		return checked{}, invalid(ReasonUnbound, "the nonce was not issued to this account; sign the nonce GET /v1/billing/crypto/gateway returns, and send its salt")
	}

	digest, err := c.Domain().Digest(p)
	if err != nil {
		return checked{}, invalid(ReasonPayload, "domain: %v", err)
	}
	signer, err := eip3009.Recover(digest, sub.Signature)
	if err != nil {
		return checked{}, invalid(ReasonSignature, "signature: %v", err)
	}
	if signer != p.From {
		return checked{}, invalid(ReasonSignature, "the signature is not %s's over this payment on %s", eip3009.Hex(p.From[:]), c.ID)
	}

	sig, err := eip3009.Canonical(sub.Signature)
	if err != nil {
		return checked{}, invalid(ReasonSignature, "signature: %v", err)
	}

	return checked{
		chain:  c,
		after:  p.ValidAfter,
		before: p.ValidBefore,
		auth: eip3009.Authorization{
			From:        eip3009.Hex(p.From[:]),
			To:          eip3009.Hex(p.To[:]),
			Value:       p.Value.String(),
			ValidAfter:  strconv.FormatUint(p.ValidAfter, 10),
			ValidBefore: strconv.FormatUint(p.ValidBefore, 10),
			Nonce:       eip3009.Hex(p.Nonce[:]),
		},
		salt:  eip3009.Hex(salt[:]),
		sig:   sig,
		cents: cents.Int64(),
	}, nil
}

func lookup(chains []Chain, id string) (Chain, bool) {
	for _, c := range chains {
		if c.ID == id {
			return c, true
		}
	}
	return Chain{}, false
}
