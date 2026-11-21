// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// A challenge is for the caller and the resource it was issued to: the route and a keyed
// tag of the caller ride in opaque, which the binding covers, and are checked when the
// credential comes back.
func TestGateBindsRouteAndCaller(t *testing.T) {
	r := newRig(t, func(c *Config) {
		c.Replay = 1 << 20
		c.Caller = func(req *http.Request) (Caller, error) {
			return Caller{Org: "acme", Subject: req.Header.Get("X-Subject")}, nil
		}
	})
	as := func(who string, h http.Header) http.Header {
		h.Set("X-Subject", who)
		return h
	}
	ch := r.challenge("GET", "/a", as("alice", http.Header{}), "")[0]
	members, err := Opaque(ch.Opaque)
	if err != nil || members["route"] != "GET /a" || len(members["nonce"]) != 22 || len(members["caller"]) != 22 {
		t.Fatalf("opaque %v %v", members, err)
	}
	if strings.Contains(ch.Opaque, "alice") || strings.Contains(rawDecode(t, ch.Opaque), "alice") {
		t.Fatal("the caller's identity is readable in the challenge")
	}
	c := credential(ch, proof).String()
	for name, req := range map[string]struct {
		method, path, who string
	}{
		"another resource, same price": {"GET", "/b", "alice"},
		"another method":               {"POST", "/a", "alice"},
		"another caller":               {"GET", "/a", "bob"},
		"no caller":                    {"GET", "/a", ""},
	} {
		wantProblem(t, r.do(req.method, req.path, as(req.who, http.Header{"Authorization": {c}}), ""), 402, Invalid)
		_ = name
	}
	if r.invoice.count() != 0 || r.hits.Load() != 0 {
		t.Fatal("a challenge was redeemed for what it was not issued for")
	}
	// None of those spent it: the caller and the resource it was for pay.
	h := as("alice", http.Header{"Authorization": {c}, "Idempotency-Key": {"k"}})
	if resp := r.do("GET", "/a", h, ""); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	// Nor does another caller get the kept response by echoing the challenge and the key.
	h = as("bob", http.Header{"Authorization": {c}, "Idempotency-Key": {"k"}})
	wantProblem(t, r.do("GET", "/a", h, ""), 402, Invalid)
	// The members are the gate's.
	for _, key := range []string{"nonce", "route", "caller"} {
		r2 := newRig(t, func(c *Config) { c.Methods = []Method{&reserved{invoice: &invoice{}, key: key}} })
		wantProblem(t, r2.do("GET", "/resource", nil, ""), 500, Internal)
	}
}

type reserved struct {
	*invoice
	key string
}

func (n *reserved) Propose(ctx context.Context, c Caller, q Quote) (*Proposal, error) {
	p, err := n.invoice.Propose(ctx, c, q)
	if p != nil {
		p.Opaque = map[string]string{n.key: "x"}
	}
	return p, err
}

func rawDecode(t *testing.T, s string) string {
	t.Helper()
	b, err := raw.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
