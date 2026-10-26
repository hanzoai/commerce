// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/payment/providers/circle"
)

// Treasury is Hanzo's Gateway position, for operators: what the treasury
// holds on each domain, what is on its way in, and what has been credited to
// customers on payments that have not landed yet.
//
// OUTBOUND (designed, not built). Paying out of this balance — to a customer,
// or to fund a customer's own wallet programmatically — is a Gateway transfer:
//
//  1. Build a burn intent: TransferSpec{sourceDomain, destinationDomain,
//     sourceContract (GatewayWallet), destinationContract (GatewayMinter),
//     sourceToken, destinationToken (USDC on each), sourceDepositor =
//     sourceSigner = the treasury, destinationRecipient, value, salt}, with
//     maxBlockHeight from /v1/info's burnIntentExpirationHeight plus margin
//     and maxFee from POST /v1/estimate (Client.Estimate).
//  2. Sign it as EIP-712 under the "GatewayWallet" domain (version 1 — NOT
//     the "GatewayWalletBatched" domain inbound payments use) by the treasury
//     MPC wallet: cloud POST /v1/wallet/{id}/sign over the digest, network
//     "evm". The ring holds the key; nothing here does.
//  3. POST /v1/transfer → attestation + operator signature (+ transferId).
//  4. Mint on the destination: either call GatewayMinter.gatewayMint with the
//     attestation (a transaction from a funded account), or ask Gateway's
//     Forwarding Service to (enableForwarder=true, a $0.05 + gas fee).
//  5. Follow GET /v1/transfer/{id} (Client.Transfer) to finalized.
//
// Each payout is a durable record keyed on its own idempotency key BEFORE the
// burn is signed, exactly as a remittance is keyed before it is settled, and a
// human-approved limit gates it: outbound moves the company's money, so it is
// never reachable from a customer's request. Fees: 0.5 bp cross-chain plus the
// source chain's gas fee; same-chain withdrawals pay gas only.
type Treasury struct {
	Network  string    `json:"network"` // mainnet | testnet
	Address  string    `json:"address"`
	Token    string    `json:"token"`
	Total    string    `json:"total"` // available, all domains, decimal USDC
	Balances []Holding `json:"balances"`
	Arriving []Arrival `json:"arriving"`
	// Credited is what customers hold, in cents, against payments Gateway
	// accepted and has not settled into the treasury yet.
	Credited int64 `json:"creditedCents"`
	Open     int   `json:"open"`
}

// Holding is the treasury's available balance on one domain.
type Holding struct {
	Domain  uint32 `json:"domain"`
	Chain   string `json:"chain"`
	Balance string `json:"balance"` // decimal USDC
}

// Arrival is a deposit into the treasury's Gateway balance not yet finalized.
type Arrival struct {
	Domain uint32 `json:"domain"`
	Chain  string `json:"chain"`
	Amount string `json:"amount"` // decimal USDC
	Tx     string `json:"tx"`
}

// Treasury reads the treasury's Gateway position live. Every figure is
// Gateway's or the ledger's; a read that fails is an error, never a zero.
func (s *Service) Treasury(ctx context.Context) (Treasury, error) {
	if !s.Armed() {
		return Treasury{}, errClosed
	}
	info, err := s.client.Info(ctx)
	if err != nil {
		return Treasury{}, shut(ReasonUnavailable, "Circle Gateway is not answering", err)
	}
	names := map[uint32]string{}
	for _, d := range info.Domains {
		names[d.Domain] = strings.TrimSpace(d.Chain + " " + d.Network)
	}
	bals, err := s.client.Balances(ctx, circle.USDC, s.cfg.Treasury)
	if err != nil {
		return Treasury{}, shut(ReasonUnavailable, "Circle Gateway did not return the treasury's balances", err)
	}
	deps, err := s.client.Deposits(ctx, circle.USDC, s.cfg.Treasury)
	if err != nil {
		return Treasury{}, shut(ReasonUnavailable, "Circle Gateway did not return the treasury's pending deposits", err)
	}

	t := Treasury{Network: string(s.cfg.Network), Address: s.cfg.Treasury, Token: circle.USDC, Balances: []Holding{}, Arriving: []Arrival{}}
	total := new(big.Int)
	for _, b := range bals {
		units, err := decimal(b.Balance, 6)
		if err != nil {
			return Treasury{}, fmt.Errorf("gateway: treasury balance on domain %d: %w", b.Domain, err)
		}
		total.Add(total, units)
		t.Balances = append(t.Balances, Holding{Domain: b.Domain, Chain: names[b.Domain], Balance: b.Balance})
	}
	sort.Slice(t.Balances, func(i, j int) bool { return t.Balances[i].Domain < t.Balances[j].Domain })
	t.Total = format(total, 6)
	for _, d := range deps {
		units, ok := new(big.Int).SetString(d.Amount, 10)
		if !ok {
			return Treasury{}, fmt.Errorf("gateway: pending deposit %s amount %q", d.TransactionHash, d.Amount)
		}
		t.Arriving = append(t.Arriving, Arrival{Domain: d.Domain, Chain: names[d.Domain], Amount: format(units, 6), Tx: d.TransactionHash})
	}

	var open []*remittance.Remittance
	if _, err := remittance.Query(systemDB()).Filter("Status=", remittance.Accepted).GetAll(&open); err != nil {
		return Treasury{}, fmt.Errorf("gateway: list accepted remittances: %w", err)
	}
	for _, r := range open {
		if r.Credited && !r.Reversed {
			t.Credited += r.AmountCents
		}
	}
	t.Open = len(open)
	return t, nil
}

// decimal reads Gateway's decimal token amount ("12.5", "12.500000") as base
// units at the token's decimals, exactly. More fractional digits than the token
// has is an error, never a rounding.
func decimal(s string, decimals int) (*big.Int, error) {
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" || !numeric(whole) || (frac != "" && !numeric(frac)) || len(frac) > decimals {
		return nil, fmt.Errorf("%q is not a decimal with at most %d places", s, decimals)
	}
	v, _ := new(big.Int).SetString(whole+frac+strings.Repeat("0", decimals-len(frac)), 10)
	return v, nil
}

func numeric(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// format renders base units as a decimal with exactly decimals places.
func format(v *big.Int, decimals int) string {
	s := v.String()
	if len(s) <= decimals {
		s = strings.Repeat("0", decimals-len(s)+1) + s
	}
	return s[:len(s)-decimals] + "." + s[len(s)-decimals:]
}
