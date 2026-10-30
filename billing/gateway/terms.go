// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/luxfi/crypto"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/eip3009"
)

// Terms is everything a payer needs to sign a top-up, x402-style: where to
// pay, the networks and token, the token's EIP-712 domain per network, the
// types, the limits, and the nonce to sign.
//
// The payer signs, for one network of Accepts:
//
//	domain      Accepts[i].Domain   (the USDC contract's own)
//	types       Types (EIP712Domain + TransferWithAuthorization)
//	primaryType TransferWithAuthorization
//	message     {from: <payer>, to: PayTo, value: <base units>,
//	             validAfter: <unix s, not after now, at most MaxAge ago>,
//	             validBefore: <unix s, between now+MinValiditySeconds and
//	             now+MaxValiditySeconds>, nonce: Nonce}
//
// and POSTs {network, salt: Salt, authorization: message, signature}. The
// payer holds the USDC in its own wallet and needs no gas: the relayer submits
// the authorization.
type Terms struct {
	Version     int                        `json:"x402Version"`
	Scheme      string                     `json:"scheme"`
	PayTo       string                     `json:"payTo"`
	Nonce       string                     `json:"nonce"`
	Salt        string                     `json:"salt"`
	MinCents    int64                      `json:"minCents"`
	MaxCents    int64                      `json:"maxCents"`
	MaxValidity int64                      `json:"maxValiditySeconds"`
	Primary     string                     `json:"primaryType"`
	Types       map[string][]eip3009.Field `json:"types"`
	Accepts     []Accept                   `json:"accepts"`
}

// Accept is one network the payment may be signed on.
type Accept struct {
	Network     string         `json:"network"`
	Asset       string         `json:"asset"`
	Symbol      string         `json:"symbol"`
	Decimals    int            `json:"decimals"`
	MinAmount   string         `json:"minAmount"` // base units
	MaxAmount   string         `json:"maxAmount"` // base units
	MinValidity int64          `json:"minValiditySeconds"`
	Domain      eip3009.Domain `json:"domain"`
}

// usdcDecimals is USDC's scale on every network in the table (ready checks the
// chain agrees).
const usdcDecimals = 6

// Terms answers the payment terms for this caller, with a nonce issued to
// them. Every call issues a fresh nonce; any of them may be signed. Only the
// networks that pass their check now are offered.
func (s *Service) Terms(ctx context.Context, org *organization.Organization, payer string) (Terms, error) {
	if err := s.caller(org, payer); err != nil {
		return Terms{}, err
	}
	if err := s.budget("terms", org.Name); err != nil {
		return Terms{}, err
	}
	var salt [32]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return Terms{}, fmt.Errorf("gateway: salt: %w", err)
	}
	nonce := bind(org.Name, payer, salt)
	t := Terms{
		Version: 1, Scheme: "exact", PayTo: s.cfg.Treasury,
		Nonce: eip3009.Hex(nonce[:]), Salt: eip3009.Hex(salt[:]),
		MinCents: MinCents, MaxCents: MaxCents, MaxValidity: int64(MaxValidity / time.Second),
		Primary: eip3009.Primary, Types: eip3009.Types(), Accepts: []Accept{},
	}
	unit := centUnit(usdcDecimals)
	var last error
	for _, c := range s.cfg.Chains {
		if _, err := s.ready(ctx, c.ID); err != nil {
			last = err
			continue
		}
		t.Accepts = append(t.Accepts, Accept{
			Network: c.ID, Asset: c.USDC, Symbol: "USDC", Decimals: usdcDecimals,
			MinAmount:   new(big.Int).Mul(big.NewInt(MinCents), unit).String(),
			MaxAmount:   new(big.Int).Mul(big.NewInt(MaxCents), unit).String(),
			MinValidity: int64(MinValidity / time.Second),
			Domain:      c.Domain(),
		})
	}
	if len(t.Accepts) == 0 {
		return Terms{}, last
	}
	return t, nil
}

// Networks lists the CAIP-2 networks the rail accepts USDC on, for the crypto
// options menu: the ones whose last check passed, from what is in hand and
// never by dialling, because the menu also serves the deposit-address rail and
// a slow node must not slow that. The reconciler keeps the checks fresh. Nil
// when the rail is shut.
func (s *Service) Networks() []string {
	if !s.Armed() {
		return nil
	}
	var out []string
	for _, c := range s.cfg.Chains {
		l := s.links[c.ID]
		l.mu.Lock()
		ok := l.ok && s.now().Sub(l.checked) < time.Hour
		l.mu.Unlock()
		if ok {
			out = append(out, c.ID)
		}
	}
	return out
}

// centUnit is how many base units make one cent: 10^(decimals-2). USDC has 6
// decimals, so 10,000.
func centUnit(decimals int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals-2)), nil)
}

// bind is the nonce issued to one caller:
//
//	keccak256("hanzo/gateway/remittance" ‖ 0x00 ‖ org ‖ 0x00 ‖ payer ‖ 0x00 ‖ salt)
//
// The payer signs it, so the signature commits to WHO is credited as well as
// to what is paid. A payment presented by a caller it was not issued to fails
// the check in verify, because no salt makes another caller's preimage hash to
// it. It needs no stored state and no secret: the salt travels with the
// payment, and keccak's preimage resistance is the whole argument. Org names
// and billing subjects carry no NUL byte, so the separators are unambiguous.
func bind(org, payer string, salt [32]byte) [32]byte {
	var out [32]byte
	copy(out[:], crypto.Keccak256(
		[]byte("hanzo/gateway/remittance\x00"), []byte(org), []byte{0},
		[]byte(payer), []byte{0}, salt[:],
	))
	return out
}
