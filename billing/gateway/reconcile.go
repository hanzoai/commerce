// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/hanzoai/commerce/models/remittance"
)

// The reconciler follows every open remittance to the end the chain gives it:
// a relayed transaction looked up and sent again if it was dropped, a use
// followed until it is deep enough to credit, a credit that did not land landed.
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
//	confirming  used on chain: credited once deep enough, or landed if the
//	            credit did not finish — first, because this is money already
//	            in the treasury
//	pending     looked up on chain, and relayed again if the node dropped it
//	refused     watched for a use until it expires, because the payer can hand
//	            the same authorization to the token directly
//
// Within each, rows are taken DUE FIRST, at most batch of them: every row the
// pass touches without progress is pushed back by a doubling wait (backoff),
// so rows that cannot move yet sink below the ones that can and never starve
// them. The pass also keeps each network's check fresh, which is what the
// options menu reads.
func (s *Service) Reconcile(ctx context.Context) {
	for _, c := range s.cfg.Chains {
		_, _ = s.ready(ctx, c.ID)
	}
	due := stamp(s.now())
	for _, status := range []string{remittance.Confirming, remittance.Pending, remittance.Refused} {
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
		if !x.Open() || x.Due == never {
			return false
		}
		x.Tries++
		x.Due = stamp(s.now().Add(backoff(x.Tries)))
		return true
	}); err != nil {
		slog.Warn("gateway: reconcile: reschedule", "remittance", id, "err", err)
	}
}

// advance moves one remittance as far as the chain says it has gone, and
// reschedules it if that was nowhere.
func (s *Service) advance(ctx context.Context, id string) {
	r, err := load(id)
	if err != nil {
		slog.Warn("gateway: reconcile: read remittance", "remittance", id, "err", err)
		return
	}
	l, err := s.ready(ctx, r.Network)
	if err != nil {
		s.later(id)
		return
	}
	mark := func(x *remittance.Remittance) string {
		return x.Status + "|" + x.TxHash + "|" + strconv.FormatInt(x.Confirmations, 10) + "|" +
			strconv.FormatBool(x.Credited) + "|" + strconv.FormatBool(x.Landed) + "|" + strconv.Itoa(len(x.Sends))
	}
	was := mark(r)
	switch r.Status {
	case remittance.Confirming:
		s.confirm(ctx, l, r)
	case remittance.Pending:
		if _, err := s.drive(ctx, l, r); err != nil && StatusOf(err) >= 500 {
			slog.Warn("gateway: reconcile: drive", "remittance", id, "err", err)
		}
	case remittance.Refused:
		s.watch(ctx, l, r)
	}
	if now, err := load(id); err == nil && mark(now) == was {
		s.later(id)
	}
}

// watch looks a refused payment up on chain: used, it is followed like any
// other; expired unused, it fails and is not looked at again. It is never
// sent again from here: presenting it again is the payer's call.
func (s *Service) watch(ctx context.Context, l *link, r *remittance.Remittance) {
	used, err := l.used(ctx, r.From, r.Nonce)
	switch {
	case err != nil:
		return
	case used:
		s.settle(ctx, l, r)
	case !s.now().Before(before(r).Add(grace)):
		s.fail(r, ReasonExpired)
	}
}
