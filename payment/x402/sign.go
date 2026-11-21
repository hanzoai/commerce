// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/luxfi/crypto"
)

// Sign is the client half of the protocol and the mirror of Verify: it produces
// the PaymentPayload a payer sends on PAYMENT-SIGNATURE, bound to exactly the
// requirements it was offered. It signs with the same Domain.Digest that Verify
// recovers against, so the two cannot drift.
//
// nonce is the client's 32 random bytes; the window is the client's to state
// within the maxTimeoutSeconds it was offered.
func Sign(offered PaymentRequirements, key *ecdsa.PrivateKey, nonce [32]byte, validAfter, validBefore uint64) (PaymentPayload, error) {
	if key == nil {
		return PaymentPayload{}, errors.New("x402: sign requires a key")
	}
	if offered.Scheme != SchemeExact {
		return PaymentPayload{}, refuse(ReasonInvalidScheme, "scheme %q; only %q is implemented", offered.Scheme, SchemeExact)
	}
	extra := extraOf(offered)
	if extra.AssetTransferMethod != TransferEIP3009 {
		return PaymentPayload{}, refuse(ReasonInvalidScheme, "assetTransferMethod %q; only %q is implemented", extra.AssetTransferMethod, TransferEIP3009)
	}
	chain, err := ChainID(offered.Network)
	if err != nil {
		return PaymentPayload{}, err
	}
	auth := Authorization{
		From:        crypto.PubkeyToAddress(key.PublicKey).Hex(),
		To:          offered.PayTo,
		Value:       offered.Amount,
		ValidAfter:  strconv.FormatUint(validAfter, 10),
		ValidBefore: strconv.FormatUint(validBefore, 10),
		Nonce:       Hex(nonce[:]),
	}
	parsed, err := auth.Parse()
	if err != nil {
		return PaymentPayload{}, refuse(ReasonInvalidRequirements, "%v", err)
	}
	digest, err := Domain{Name: extra.Name, Version: extra.Version, ChainID: chain, VerifyingContract: offered.Asset}.Digest(parsed)
	if err != nil {
		return PaymentPayload{}, refuse(ReasonInvalidRequirements, "%v", err)
	}
	sig, err := crypto.Sign(digest[:], key)
	if err != nil {
		return PaymentPayload{}, fmt.Errorf("x402: sign: %w", err)
	}
	canonical, err := Canonical(Hex(sig))
	if err != nil {
		return PaymentPayload{}, fmt.Errorf("x402: sign: %w", err)
	}
	return PaymentPayload{
		X402Version: Version,
		Accepted:    offered,
		Payload:     ExactPayload{Signature: canonical, Authorization: auth},
	}, nil
}

// Payer answers a 402: it picks one of the offered requirements and returns the
// payment for it, or refuses.
type Payer interface {
	Pay(ctx context.Context, required PaymentRequired) (PaymentPayload, error)
}

// Signer is a Payer holding one key. It never spends on its own authority: Allow
// is asked about every requirement before anything is signed, and a Signer with
// no Allow signs nothing. The ceiling on what an agent may spend is the
// caller's to set, here.
type Signer struct {
	Key *ecdsa.PrivateKey
	// Allow reports whether this payer may pay the requirement. It sees the
	// resource, the asset, the network, the amount and the payee.
	Allow func(resource ResourceInfo, r PaymentRequirements) bool
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// ErrDeclined is what Pay returns when no offered requirement is one the Signer
// may, or can, pay.
var ErrDeclined = errors.New("x402: no offered payment requirement is acceptable")

// Pay signs the first offered requirement that Allow approves and that this
// package can sign (exact, eip3009, eip155). The authorization has no lower
// bound, so clock skew between payer and resource cannot refuse it, and expires
// maxTimeoutSeconds after now.
func (s Signer) Pay(_ context.Context, required PaymentRequired) (PaymentPayload, error) {
	if s.Allow == nil {
		return PaymentPayload{}, ErrDeclined
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	for _, r := range required.Accepts {
		if r.Scheme != SchemeExact || extraOf(r).AssetTransferMethod != TransferEIP3009 || r.MaxTimeoutSeconds <= 0 {
			continue
		}
		if _, err := ChainID(r.Network); err != nil || !s.Allow(required.Resource, r) {
			continue
		}
		var nonce [32]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return PaymentPayload{}, fmt.Errorf("x402: nonce: %w", err)
		}
		pay, err := Sign(r, s.Key, nonce, 0, uint64(now().Unix())+uint64(r.MaxTimeoutSeconds))
		if err != nil {
			continue
		}
		if required.Resource.URL != "" {
			res := required.Resource
			pay.Resource = &res
		}
		pay.Extensions = required.Extensions
		return pay, nil
	}
	return PaymentPayload{}, ErrDeclined
}
