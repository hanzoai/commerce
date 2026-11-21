// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"net/http"
	"strings"
	"testing"
)

// A challenge is bound to the route and the caller it was issued to: both ride in opaque,
// which the binding covers, and are checked when the credential comes back.
func TestGateBindsRouteAndCaller(t *testing.T) {
	r := newRig(t, func(c *Config) {
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
	if err != nil || members["route"] != "GET /a" || len(members["nonce"]) != 43 || len(members["caller"]) != 43 {
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
}

func rawDecode(t *testing.T, s string) string {
	t.Helper()
	b, err := raw.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
