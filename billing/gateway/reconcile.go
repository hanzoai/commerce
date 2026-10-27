// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"
	"log/slog"
	"strconv"
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

// Reconcile is one pass, in the order that matters most to money:
//
//	accepted  credited (or deferred) and read back from Gateway until it
//	          completes or fails — first, because a failure here is credit
//	          to take back
//	pending   looked up, and sent again only if Gateway holds nothing
//	refused   looked up by nonce until it expires, because a payment Gateway
//	          refused can be handed to Gateway directly later, and the money
//	          would then be the treasury's
//
// Within each, rows are taken DUE FIRST, at most batch of them: every row the
// pass touches without progress is pushed back by a doubling wait (backoff),
// so a few hundred rows that cannot move yet sink below the ones that can and
// never starve them. The pass also keeps Gateway's network table warm, which
// is what the options menu reads.
func (s *Service) Reconcile(ctx context.Context) {
	_, _ = s.networks(ctx)
	due := stamp(s.now())
	for _, status := range []string{remittance.Accepted, remittance.Pending, remittance.Refused} {
		keys, err := remittance.Query(systemDB()).Filter("Status=", status).Filter("Due<=", due).
			Order("Due").Limit(batch).GetKeys()
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

// later pushes a row the pass could not move to the back of the queue.
func (s *Service) later(id string) {
	if _, err := update(id, func(x *remittance.Remittance) bool {
		if (!x.Open() && x.Status != remittance.Refused) || x.Due == never {
			return false
		}
		x.Tries++
		x.Due = stamp(s.now().Add(backoff(x.Tries)))
		return true
	}); err != nil {
		slog.Warn("gateway: reconcile: reschedule", "remittance", id, "err", err)
	}
}

// advance moves one remittance as far as Gateway says it has gone, and
// reschedules it if that was nowhere.
func (s *Service) advance(ctx context.Context, id string) {
	r, err := load(id)
	if err != nil {
		slog.Warn("gateway: reconcile: read remittance", "remittance", id, "err", err)
		return
	}
	before := r.Status + "|" + r.State + "|" + strconv.FormatBool(r.Landed)
	switch r.Status {
	case remittance.Pending:
		// A remittance recorded and never sent (the process stopped between
		// the two) is sent; one sent before is looked up first. drive knows
		// which.
		if _, err := s.drive(ctx, r); err != nil && StatusOf(err) >= 500 {
			slog.Warn("gateway: reconcile: drive", "remittance", id, "err", err)
		}
	case remittance.Refused:
		s.sweep(ctx, r)
	case remittance.Accepted:
		r = s.credit(ctx, r)
		p, err := s.client.Payment(ctx, r.Payment)
		if err != nil {
			slog.Warn("gateway: reconcile: read payment", "payment", r.Payment, "remittance", id, "err", err)
			break
		}
		if !matches(r, p) {
			slog.Error("RECONCILE: gateway: Circle's payment does not match the remittance; left as it stands", "payment", p.ID, "remittance", id, "from", p.From, "to", p.To, "amount", p.Amount, "nonce", p.Nonce)
			break
		}
		if p.Status == circle.Failed {
			slog.Error("RECONCILE: gateway: Circle FAILED a payment after accepting it; reversing any credit", "payment", p.ID, "remittance", id, "cents", r.AmountCents, "org", r.Org, "payer", r.Payer)
			if _, err := s.fail(ctx, r, "failed"); err != nil {
				slog.Warn("gateway: reconcile: fail", "remittance", id, "err", err)
			}
			break
		}
		if _, err := s.accept(ctx, r, p); err != nil {
			slog.Warn("gateway: reconcile: accept", "remittance", id, "err", err)
		}
	}
	if now, err := load(id); err == nil && now.Status+"|"+now.State+"|"+strconv.FormatBool(now.Landed) == before {
		s.later(id)
	}
}

// sweep looks a refused payment up by nonce: if Gateway now holds it — the
// payer handed it to Gateway directly — it is adopted and credited; if Gateway
// holds another payment under the nonce it can never settle; once it has
// expired unseen it is not looked at again.
func (s *Service) sweep(ctx context.Context, r *remittance.Remittance) {
	mine, other, err := s.find(ctx, r)
	switch {
	case err != nil:
		return
	case mine != nil:
		if _, err := s.adopt(ctx, r, *mine); err != nil {
			slog.Warn("gateway: reconcile: adopt refused", "remittance", r.Id(), "err", err)
		}
	case other:
		_, _ = s.fail(ctx, r, ReasonConflict)
	case s.expired(r):
		_, _ = update(r.Id(), func(x *remittance.Remittance) bool {
			if x.Status != remittance.Refused || x.Due == never {
				return false
			}
			x.Due = never
			return true
		})
	}
}
