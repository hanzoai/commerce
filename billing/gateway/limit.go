// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/remittance"
)

// What bounds the rail's cost to an org that is free to make: a request budget
// per org (limiter), one payment in flight per paying wallet (lane), and one
// relayer transaction drafted at a time across replicas (lease).

// limiter is a fixed one-minute window per key. Per process: with N replicas
// an org gets at most N times the budget, which still bounds it.
type limiter struct {
	mu   sync.Mutex
	seen map[string]tally
}

// tally is one key's requests in the minute that started at start.
type tally struct {
	start time.Time
	n     int
}

// allow counts one request for key at now and reports whether it fits in the
// minute's budget.
func (l *limiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.seen == nil {
		l.seen = map[string]tally{}
	}
	w := l.seen[key]
	if now.Sub(w.start) >= time.Minute {
		w = tally{start: now}
	}
	if w.n >= PerMinute {
		return false
	}
	w.n++
	l.seen[key] = w
	if len(l.seen) > 10_000 {
		for k, v := range l.seen {
			if now.Sub(v.start) >= time.Minute {
				delete(l.seen, k)
			}
		}
	}
	return true
}

func (s *Service) budget(kind, org string) error {
	if !s.rate.allow(kind+"\x00"+org, s.now()) {
		return refuse(http.StatusTooManyRequests, ReasonRate, "too many %s requests from this account; try again in a minute", kind)
	}
	return nil
}

func newSelf() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// lock applies fn to the lock row id, writing only over the row read, and
// reports what fn decided. A row that does not exist is read as free.
func lock(id string, fn func(*remittance.Lock) bool) (bool, error) {
	db := systemDB()
	for range mixin.Swaps {
		l := remittance.NewLock(db)
		switch err := l.Get(db.NewKey(l.Kind(), id, 0, nil)); {
		case errors.Is(err, datastore.ErrNoSuchEntity):
			l = remittance.NewLock(db)
			l.SetId(id)
		case err != nil:
			return false, err
		}
		if !fn(l) {
			return false, nil
		}
		ok, err := l.Swap()
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, fmt.Errorf("gateway: lock %s changed under %d reads in a row", id, mixin.Swaps)
}

// lane takes the paying wallet's one slot for r: a wallet has at most one
// payment whose relayer transaction is out and not yet resolved, so a payment
// it cannot fund costs the relayer at most one failed transaction at a time,
// never one per payment. A slot whose holder is no longer in flight is taken
// over. It reports whether r holds the slot.
func (s *Service) lane(r *remittance.Remittance) (bool, error) {
	id := "lane\x00" + r.Network + "\x00" + r.From
	held := false
	_, err := lock(id, func(l *remittance.Lock) bool {
		held = false
		if l.Holder == r.Id() {
			held = true
			return false // held already: nothing to write
		}
		if l.Holder != "" {
			if h, err := load(l.Holder); err == nil && inFlight(h) {
				return false
			}
		}
		l.Holder, held = r.Id(), true
		return true
	})
	return held, err
}

// holds reports whether r holds its wallet's slot now.
func (s *Service) holds(r *remittance.Remittance) bool {
	db := systemDB()
	l := remittance.NewLock(db)
	if err := l.Get(db.NewKey(l.Kind(), "lane\x00"+r.Network+"\x00"+r.From, 0, nil)); err != nil {
		return false
	}
	return l.Holder == r.Id()
}

// inFlight reports whether a remittance has a relayer transaction out that the
// chain has not resolved: pending, and sent.
func inFlight(r *remittance.Remittance) bool {
	return r.Status == remittance.Pending && len(r.Sends) > 0
}

// leaseFor is how long one replica holds the relayer while it drafts, signs,
// records and broadcasts a transaction: long enough for that, short enough
// that a replica that died holding it costs a payment one pass.
const leaseFor = 30 * time.Second

// lease takes the relayer key on network for this process, or reports that
// another replica holds it.
func (s *Service) lease(network string) (bool, error) {
	id := "relayer\x00" + network + "\x00" + s.cfg.RelayerAddress()
	now := s.now()
	took := false
	_, err := lock(id, func(l *remittance.Lock) bool {
		took = false
		if l.Holder != "" && l.Holder != s.self && now.Before(l.Until) {
			return false
		}
		l.Holder, l.Until, took = s.self, now.Add(leaseFor), true
		return true
	})
	return took, err
}

// release lets go of the relayer key on network, if this process holds it.
func (s *Service) release(network string) {
	id := "relayer\x00" + network + "\x00" + s.cfg.RelayerAddress()
	_, _ = lock(id, func(l *remittance.Lock) bool {
		if l.Holder != s.self {
			return false
		}
		l.Holder, l.Until = "", time.Time{}
		return true
	})
}
