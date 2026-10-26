// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"
	"log/slog"
	"time"

	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/payment/providers/circle"
)

// The reconciler follows every open remittance to the end Gateway gives it.
//
// It POLLS, and that is not a stand-in for a webhook. Gateway's notifications
// are gateway.deposit.finalized, gateway.mint.finalized and
// gateway.mint.forwarded — deposits into a Gateway Wallet and mints on a
// destination chain. None reports a batched payment being batched, confirmed
// or failed, so no subscription, signed or not, could tell this rail what it
// needs to know; GET /v1/x402/transfers/{id} can. Polling also needs no
// inbound endpoint and no Circle API key, which is two fewer things to defend.
//
// A pass is idempotent — every transition is a swap over the row it read, and
// every credit is keyed on the remittance — so it runs on every replica with
// no lock and no leader.

// batch bounds one pass per status. A backlog larger than this drains over
// successive passes rather than in one long one.
const batch = 200

// Start runs the schedule. A no-op when the rail is not armed or is already
// running; Stop ends it.
func (s *Service) Start() {
	if !s.Armed() {
		return
	}
	s.run.Lock()
	defer s.run.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.cancel, s.done = cancel, done
	go s.loop(ctx, done)
}

// Stop ends the schedule and waits for the pass in flight.
func (s *Service) Stop() {
	if s == nil {
		return
	}
	s.run.Lock()
	cancel, done := s.cancel, s.done
	s.cancel, s.done = nil, nil
	s.run.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
}

// Running reports whether the schedule is turning.
func (s *Service) Running() bool {
	if s == nil {
		return false
	}
	s.run.Lock()
	defer s.run.Unlock()
	return s.cancel != nil
}

func (s *Service) loop(ctx context.Context, done chan struct{}) {
	defer close(done)
	t := time.NewTicker(s.every)
	defer t.Stop()
	for {
		// At once on start: a restart must pick up what was left open, not
		// wait out an interval first.
		s.Reconcile(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Reconcile is one pass: every pending remittance is looked up (and sent again
// if Gateway has none), and every accepted one is credited if it is not yet
// and read back from Gateway until it settles or fails.
func (s *Service) Reconcile(ctx context.Context) {
	for _, status := range []string{remittance.Pending, remittance.Accepted} {
		keys, err := remittance.Query(systemDB()).Filter("Status=", status).Limit(batch).GetKeys()
		if err != nil {
			slog.Error("gateway: reconcile: list remittances", "status", status, "err", err)
			continue
		}
		for _, k := range keys {
			if ctx.Err() != nil {
				return
			}
			s.advance(ctx, k.StringID())
		}
	}
}

// advance moves one remittance as far as Gateway says it has gone.
func (s *Service) advance(ctx context.Context, id string) {
	r, err := load(id)
	if err != nil {
		slog.Warn("gateway: reconcile: read remittance", "remittance", id, "err", err)
		return
	}
	switch r.Status {
	case remittance.Pending:
		// A remittance recorded and never sent (the process stopped between
		// the two) is sent; one sent before is looked up first. drive knows
		// which.
		if _, err := s.drive(ctx, r); err != nil && StatusOf(err) >= 500 {
			slog.Warn("gateway: reconcile: drive", "remittance", id, "err", err)
		}
	case remittance.Accepted:
		r = s.credit(ctx, r)
		p, err := s.client.Payment(ctx, r.Payment)
		if err != nil {
			slog.Warn("gateway: reconcile: read payment", "payment", r.Payment, "remittance", id, "err", err)
			return
		}
		if !matches(r, p) {
			slog.Error("RECONCILE: gateway: Circle's payment does not match the remittance; left as it stands", "payment", p.ID, "remittance", id, "from", p.From, "to", p.To, "amount", p.Amount, "nonce", p.Nonce)
			return
		}
		switch p.Status {
		case circle.Confirmed, circle.Completed:
			if _, err := s.accept(ctx, r, p.ID, remittance.Settled, p.TxHash); err != nil {
				slog.Warn("gateway: reconcile: settle", "remittance", id, "err", err)
			}
		case circle.Failed:
			slog.Error("RECONCILE: gateway: Circle FAILED a payment after accepting it; reversing the credit", "payment", p.ID, "remittance", id, "cents", r.AmountCents, "org", r.Org, "payer", r.Payer)
			if _, err := s.fail(ctx, r, "failed"); err != nil {
				slog.Warn("gateway: reconcile: fail", "remittance", id, "err", err)
			}
		}
	}
}
