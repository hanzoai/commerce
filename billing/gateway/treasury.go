// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"

	"github.com/hanzoai/commerce/models/remittance"
)

// Treasury is the rail's position, for operators: per network, what the
// treasury holds in USDC, what the relayer holds to pay gas with (its runway),
// and the payments still open.
type Treasury struct {
	Network  string    `json:"network"` // mainnet | testnet
	Treasury string    `json:"treasury"`
	Relayer  string    `json:"relayer"`
	Chains   []Holding `json:"chains"`
	Open     Open      `json:"open"`
}

// Holding is one network's balances, in base units. Error names a network that
// did not answer, so a balance the page cannot read is never shown as zero.
type Holding struct {
	Network    string `json:"network"`
	USDC       string `json:"usdc,omitempty"`
	RelayerWei string `json:"relayerWei,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Open counts the remittances not yet final, and what they would credit.
type Open struct {
	Pending    int   `json:"pending"`
	Confirming int   `json:"confirming"`
	Refused    int   `json:"refused"`
	Cents      int64 `json:"cents"`
}

// Treasury reads the position. Read-only: it signs and sends nothing.
func (s *Service) Treasury(ctx context.Context) (Treasury, error) {
	if !s.Armed() {
		return Treasury{}, errClosed
	}
	t := Treasury{Network: s.Network(), Treasury: s.cfg.Treasury, Relayer: s.cfg.RelayerAddress(), Chains: []Holding{}}
	for _, c := range s.cfg.Chains {
		h := Holding{Network: c.ID}
		l, err := s.ready(ctx, c.ID)
		if err != nil {
			h.Error = err.Error()
			t.Chains = append(t.Chains, h)
			continue
		}
		if b, err := l.balance(ctx, s.cfg.Treasury); err != nil {
			h.Error = err.Error()
		} else {
			h.USDC = b.String()
		}
		if g, err := l.rpc().NativeBalance(ctx, s.cfg.RelayerAddress()); err != nil {
			h.Error = err.Error()
		} else {
			h.RelayerWei = g.String()
		}
		t.Chains = append(t.Chains, h)
	}
	for _, status := range []string{remittance.Pending, remittance.Confirming, remittance.Refused} {
		var rows []*remittance.Remittance
		if _, err := remittance.Query(systemDB()).Filter("Status=", status).Limit(10_000).GetAll(&rows); err != nil {
			return Treasury{}, err
		}
		for _, r := range rows {
			if status != remittance.Refused {
				t.Open.Cents += r.AmountCents
			}
		}
		switch status {
		case remittance.Pending:
			t.Open.Pending = len(rows)
		case remittance.Confirming:
			t.Open.Confirming = len(rows)
		case remittance.Refused:
			t.Open.Refused = len(rows)
		}
	}
	return t, nil
}
