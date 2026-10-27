// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/luxfi/crypto"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/providers/circle"
)

// network is one chain the rail accepts USDC on, as Gateway states it live.
type network struct {
	id       string // CAIP-2, "eip155:8453"
	asset    string // USDC contract, lowercase 0x hex
	decimals int    // always 6 (readNetworks refuses anything else)
	min      time.Duration
	domain   circle.Domain
}

// readNetworks keeps the Gateway networks a USDC payment to the treasury can
// be signed and verified on, from GET /v1/x402/supported.
//
// A network is kept only when every fact the rail relies on is present and is
// the one this package signs for: the exact scheme, an eip155 chain id, the
// GatewayWalletBatched domain at version 1, a readable GatewayWallet contract,
// and USDC with 6 decimals that accepts a version-1 authorization. A network
// Gateway describes any other way is not offered: signing for a domain this
// package does not implement would take payments it cannot verify.
//
// It is a pure function of Gateway's answer so the rule is testable without
// a network.
func readNetworks(kinds []circle.Kind) []network {
	var out []network
	for _, k := range kinds {
		if k.Scheme != circle.Scheme || k.Extra.Name != circle.Name || k.Extra.Version != circle.Release {
			continue
		}
		chain, ok := chainID(k.Network)
		if !ok {
			continue
		}
		contract, err := circle.Address(k.Extra.VerifyingContract)
		if err != nil {
			continue
		}
		min := time.Duration(k.Extra.MinValidity) * time.Second
		if min <= 0 {
			min = fallbackValidity
		}
		for _, a := range k.Extra.Assets {
			if a.Symbol != circle.USDC || a.Decimals != 6 || !has(a.Versions, circle.Release) {
				continue
			}
			addr, err := circle.Address(a.Address)
			if err != nil {
				continue
			}
			out = append(out, network{
				id:       k.Network,
				asset:    circle.Hex(addr[:]),
				decimals: a.Decimals,
				min:      min,
				domain: circle.Domain{
					Name: circle.Name, Version: circle.Release, ChainID: chain,
					VerifyingContract: circle.Hex(contract[:]),
				},
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].domain.ChainID < out[j].domain.ChainID })
	return out
}

// chainID reads "eip155:<id>" with a positive decimal id and nothing else.
func chainID(caip string) (int64, bool) {
	ref, ok := strings.CutPrefix(caip, "eip155:")
	if !ok || ref == "" || ref[0] == '0' {
		return 0, false
	}
	for i := 0; i < len(ref); i++ {
		if ref[i] < '0' || ref[i] > '9' {
			return 0, false
		}
	}
	id, err := strconv.ParseInt(ref, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func has(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// networks is Gateway's network table, read live and kept for termsAge.
//
// ONE read is in flight at a time, whoever asks: the rest wait for its answer
// rather than each asking Gateway, and a failed read is remembered for
// failAge, so a slow or failing Gateway costs one call per window rather than
// one per request. A failure is an error, never an empty table that looks
// like "no networks".
func (s *Service) networks(ctx context.Context) ([]network, error) {
	for {
		s.mu.Lock()
		now := s.now()
		switch {
		case s.nets != nil && now.Sub(s.read) < termsAge:
			nets := s.nets
			s.mu.Unlock()
			return nets, nil
		case s.ferr != nil && now.Sub(s.failed) < failAge:
			err := s.ferr
			s.mu.Unlock()
			return nil, err
		case s.flight != nil:
			ch := s.flight
			s.mu.Unlock()
			select {
			case <-ch:
				continue // the read in flight answered: look again
			case <-ctx.Done():
				return nil, shut(ReasonUnavailable, "Circle Gateway is not answering; try again shortly or use a card, bank transfer or wire.", ctx.Err())
			}
		}
		ch := make(chan struct{})
		s.flight = ch
		s.mu.Unlock()

		// Read on a context of its own: the request that happened to start it
		// may be cancelled, and every waiter depends on its answer. The
		// client's own timeout bounds it.
		kinds, err := s.client.Supported(context.WithoutCancel(ctx))
		var nets []network
		if err == nil {
			if nets = readNetworks(kinds); len(nets) == 0 {
				err = fmt.Errorf("x402/supported: %d kinds, none usable", len(kinds))
			}
		}

		s.mu.Lock()
		s.flight = nil
		if err != nil {
			s.ferr, s.failed = shut(ReasonUnavailable, "Circle Gateway is not answering, or lists no network this rail can accept USDC on; try again shortly or use a card, bank transfer or wire.", err), s.now()
		} else {
			s.nets, s.read, s.ferr = nets, s.now(), nil
		}
		close(ch)
		s.mu.Unlock()
	}
}

func lookup(nets []network, id string) (network, bool) {
	for _, n := range nets {
		if n.id == id {
			return n, true
		}
	}
	return network{}, false
}

// Networks lists the CAIP-2 networks the rail accepts USDC on, for the crypto
// options menu, from the table already in hand and never from Gateway: the
// menu also serves the deposit-address rail, and a slow Gateway must not slow
// that. The reconciler keeps the table warm. Nil when the rail is shut or no
// table has been read in the last hour.
func (s *Service) Networks() []string {
	if !s.Armed() {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nets == nil || s.now().Sub(s.read) > time.Hour {
		return nil
	}
	out := make([]string, 0, len(s.nets))
	for _, n := range s.nets {
		out = append(out, n.id)
	}
	return out
}

// Terms is everything a payer needs to sign a top-up, x402-style: where to
// pay, the networks and token, the EIP-712 domain per network, the types, the
// limits, and the nonce to sign.
//
// The payer signs, for one network of Accepts:
//
//	domain      Accepts[i].Domain
//	types       Types (EIP712Domain + TransferWithAuthorization)
//	primaryType TransferWithAuthorization
//	message     {from: <payer>, to: PayTo, value: <base units>,
//	             validAfter: <unix s, not after now>, validBefore: <unix s,
//	             between now+MinValiditySeconds and now+MaxValiditySeconds>,
//	             nonce: Nonce}
//
// and POSTs {network, salt: Salt, authorization: message, signature}.
type Terms struct {
	Version     int                       `json:"x402Version"`
	Scheme      string                    `json:"scheme"`
	PayTo       string                    `json:"payTo"`
	Nonce       string                    `json:"nonce"`
	Salt        string                    `json:"salt"`
	MinCents    int64                     `json:"minCents"`
	MaxCents    int64                     `json:"maxCents"`
	MaxValidity int64                     `json:"maxValiditySeconds"`
	Primary     string                    `json:"primaryType"`
	Types       map[string][]circle.Field `json:"types"`
	Accepts     []Accept                  `json:"accepts"`
}

// Accept is one network the payment may be signed on.
type Accept struct {
	Network     string        `json:"network"`
	Asset       string        `json:"asset"`
	Symbol      string        `json:"symbol"`
	Decimals    int           `json:"decimals"`
	MinAmount   string        `json:"minAmount"` // base units
	MaxAmount   string        `json:"maxAmount"` // base units
	MinValidity int64         `json:"minValiditySeconds"`
	Domain      circle.Domain `json:"domain"`
}

// Terms answers the payment terms for this caller, with a nonce issued to
// them. Every call issues a fresh nonce; any of them may be signed.
func (s *Service) Terms(ctx context.Context, org *organization.Organization, payer string) (Terms, error) {
	if err := s.caller(org, payer); err != nil {
		return Terms{}, err
	}
	nets, err := s.networks(ctx)
	if err != nil {
		return Terms{}, err
	}
	var salt [32]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return Terms{}, fmt.Errorf("gateway: salt: %w", err)
	}
	nonce := bind(org.Name, payer, salt)
	t := Terms{
		Version: circle.Version, Scheme: circle.Scheme, PayTo: s.cfg.Treasury,
		Nonce: circle.Hex(nonce[:]), Salt: circle.Hex(salt[:]),
		MinCents: MinCents, MaxCents: MaxCents, MaxValidity: int64(MaxValidity / time.Second),
		Primary: circle.Primary, Types: circle.Types(),
	}
	for _, n := range nets {
		unit := centUnit(n.decimals)
		t.Accepts = append(t.Accepts, Accept{
			Network: n.id, Asset: n.asset, Symbol: circle.USDC, Decimals: n.decimals,
			MinAmount:   new(big.Int).Mul(big.NewInt(MinCents), unit).String(),
			MaxAmount:   new(big.Int).Mul(big.NewInt(MaxCents), unit).String(),
			MinValidity: int64(n.min / time.Second),
			Domain:      n.domain,
		})
	}
	return t, nil
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
