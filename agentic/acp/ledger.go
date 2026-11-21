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
)

// Stored is the response kept against an idempotency key.
type Stored struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
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
	// Release frees the key to run again.
	Release(ctx context.Context) error
}

// Ledger is the idempotency store of rfc.agentic_checkout.md §6. scope is the authenticated
// identity and endpoint path (§6.1), digest the fingerprint of the body (§6.2). It keeps a
// settled key at least 24 hours (§6.6).
type Ledger interface {
	Begin(ctx context.Context, scope, key, digest string) (Verdict, Stored, Lease, error)
}

// fingerprint is the digest of a JSON body under the equality of §6.2: key order and number
// spelling (1.0 and 1) are irrelevant; arrays are ordered; null differs from absent.
func fingerprint(body []byte) (string, error) {
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

// number writes a JSON number as sign, significant digits and a power of ten.
func number(s string) string {
	orig := s
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	mant, exp := s, int64(0)
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		e, err := strconv.ParseInt(s[i+1:], 10, 64)
		if err != nil {
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
