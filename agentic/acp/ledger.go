// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Retention is how long a seller keeps an idempotency key and its response: at least 24 hours
// (rfc.agentic_checkout.md §6.6).
const Retention = 24 * time.Hour

// Stored is the response kept against an idempotency key.
type Stored struct {
	Status int
	Body   []byte
}

// Verdict is what a Ledger says about a key.
type Verdict int

// Verdicts of Ledger.Begin.
const (
	// Fresh: the key is claimed for this request, which runs and then settles or releases it.
	Fresh Verdict = iota
	// Replay: the same request already ran; its stored response is answered again.
	Replay
	// Mismatch: the key was used for a different request body.
	Mismatch
	// Busy: the request that holds the key is still running.
	Busy
)

// Lease is a claimed key.
type Lease interface {
	// Settle keeps the response against the key.
	Settle(ctx context.Context, res Stored) error
	// Release drops the claim, so the key is free to run again: a 5xx is never kept
	// (rfc.agentic_checkout.md §6.5).
	Release(ctx context.Context) error
}

// Ledger is the idempotency store of rfc.agentic_checkout.md §6. scope names the
// authenticated identity and the endpoint path, so a key on another endpoint is another key;
// digest is the fingerprint of the request body (Fingerprint).
type Ledger interface {
	Begin(ctx context.Context, scope, key, digest string) (Verdict, Stored, Lease, error)
}

// Fingerprint is the digest of a JSON body under the equality of §6.2: key order is
// irrelevant, a number equals the same number written differently (1.0 and 1), an array is
// ordered, and null differs from an absent key. An empty body has its own digest.
func Fingerprint(body []byte) (string, error) {
	h := sha256.New()
	if len(bytes.TrimSpace(body)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			return "", err
		}
		if _, err := dec.Token(); err != io.EOF {
			return "", errors.New("acp: trailing data after the JSON body")
		}
		canon(h, v)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func canon(w io.Writer, v any) {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		io.WriteString(w, "{")
		for i, k := range keys {
			if i > 0 {
				io.WriteString(w, ",")
			}
			b, _ := json.Marshal(k)
			w.Write(b)
			io.WriteString(w, ":")
			canon(w, x[k])
		}
		io.WriteString(w, "}")
	case []any:
		io.WriteString(w, "[")
		for i, e := range x {
			if i > 0 {
				io.WriteString(w, ",")
			}
			canon(w, e)
		}
		io.WriteString(w, "]")
	case json.Number:
		io.WriteString(w, number(string(x)))
	default:
		b, _ := json.Marshal(x)
		w.Write(b)
	}
}

// number writes a JSON number as sign, significant digits and a power of ten, so 1.0, 1,
// 10e-1 and 0.1e1 are one value. An exponent too large to normalise is kept as written.
func number(s string) string {
	orig := s
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	mant, exp := s, int64(0)
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		e, err := strconv.ParseInt(s[i+1:], 10, 32)
		if err != nil || e > 1<<16 || e < -(1<<16) {
			return orig
		}
		mant, exp = s[:i], e
	}
	whole, frac, _ := strings.Cut(mant, ".")
	digits := strings.TrimLeft(whole+frac, "0")
	exp -= int64(len(frac))
	trimmed := strings.TrimRight(digits, "0")
	exp += int64(len(digits) - len(trimmed))
	if trimmed == "" {
		return "0"
	}
	sign := ""
	if neg {
		sign = "-"
	}
	return sign + trimmed + "e" + strconv.FormatInt(exp, 10)
}

// Memory is a Ledger held in the process: for tests and a single node. It forgets a key
// after Retention. A deployment that restarts or runs more than one node uses a durable
// Ledger (see Commerce).
type Memory struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]*entry
	swept   time.Time
}

type entry struct {
	digest  string
	res     *Stored
	expires time.Time
}

// NewMemory returns an empty in-process Ledger.
func NewMemory() *Memory {
	return &Memory{now: time.Now, entries: map[string]*entry{}}
}

// Begin implements Ledger.
func (m *Memory) Begin(_ context.Context, scope, key, digest string) (Verdict, Stored, Lease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if now.Sub(m.swept) >= time.Minute {
		for k, e := range m.entries {
			if now.After(e.expires) {
				delete(m.entries, k)
			}
		}
		m.swept = now
	}
	id := scope + "\x00" + key
	if e, ok := m.entries[id]; ok && !now.After(e.expires) {
		switch {
		case e.res == nil:
			return Busy, Stored{}, nil, nil
		case e.digest != digest:
			return Mismatch, Stored{}, nil, nil
		default:
			return Replay, *e.res, nil, nil
		}
	}
	e := &entry{digest: digest, expires: now.Add(Retention)}
	m.entries[id] = e
	return Fresh, Stored{}, &memoryLease{m: m, id: id, e: e}, nil
}

type memoryLease struct {
	m  *Memory
	id string
	e  *entry
}

func (l *memoryLease) Settle(_ context.Context, res Stored) error {
	l.m.mu.Lock()
	defer l.m.mu.Unlock()
	l.e.res = &res
	return nil
}

func (l *memoryLease) Release(_ context.Context) error {
	l.m.mu.Lock()
	defer l.m.mu.Unlock()
	if l.m.entries[l.id] == l.e {
		delete(l.m.entries, l.id)
	}
	return nil
}
