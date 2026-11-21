// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

// The httptest certificate is valid for example.com; the client dials the test
// server whatever the host, so a profile URL can carry a real host name.
const profileURL = "https://example.com/.well-known/ucp"

func fetcherFor(t *testing.T, h http.Handler, cfg FetcherConfig) *Fetcher {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	tr := srv.Client().Transport.(*http.Transport).Clone()
	tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}
	cfg.Client = &http.Client{Transport: tr}
	return NewFetcher(cfg)
}

func profileJSON(t *testing.T, p *Profile) []byte {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func TestFetcherFetchesValidatesCachesAndAdmits(t *testing.T) {
	p := platformWithSpec(t)
	p.UCP.Capabilities["com.example.loyalty"] = []Capability{{Entity: Entity{Version: "2026-06-15", Spec: "https://example.com/s", Schema: "https://evil.example/l.json"}}}
	var hits atomic.Int32
	clk := &fakeClock{now: t0}
	f := fetcherFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Accept") != "application/json" || r.URL.Path != "/.well-known/ucp" {
			t.Errorf("unexpected request %s %s", r.URL.Path, r.Header.Get("Accept"))
		}
		w.Write(profileJSON(t, p))
	}), FetcherConfig{Now: clk.Now})

	got, err := f.Platform(context.Background(), profileURL, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.UCP.Capabilities["com.example.loyalty"]; ok {
		t.Fatal("an entity whose schema does not bind was admitted")
	}
	if _, ok := got.UCP.Capabilities[CapCheckout]; !ok {
		t.Fatal("checkout is missing")
	}
	if _, err := f.Platform(context.Background(), profileURL, false); err != nil || hits.Load() != 1 {
		t.Fatalf("second call: %v, %d fetches; a fresh profile is cached", err, hits.Load())
	}
	clk.now = t0.Add(59 * time.Second)
	f.Platform(context.Background(), profileURL, false)
	if hits.Load() != 1 {
		t.Fatal("refetched inside the TTL floor")
	}
	clk.now = t0.Add(61 * time.Second)
	if _, err := f.Platform(context.Background(), profileURL, false); err != nil || hits.Load() != 2 {
		t.Fatalf("after the TTL: %v, %d fetches", err, hits.Load())
	}
}

func TestForcedRefreshHappensOncePerTTL(t *testing.T) {
	p := platformWithSpec(t)
	var hits atomic.Int32
	clk := &fakeClock{now: t0}
	f := fetcherFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write(profileJSON(t, p))
	}), FetcherConfig{Now: clk.Now})
	f.Platform(context.Background(), profileURL, false) // 1
	f.Platform(context.Background(), profileURL, true)  // 2: the first forced refresh
	f.Platform(context.Background(), profileURL, true)  // served from cache: one forced refresh per TTL per origin
	if hits.Load() != 2 {
		t.Fatalf("%d fetches; want 2", hits.Load())
	}
	clk.now = t0.Add(61 * time.Second)
	f.Platform(context.Background(), profileURL, true)
	if hits.Load() != 3 {
		t.Fatalf("%d fetches; want 3", hits.Load())
	}
}

func TestCacheControlMaxAgeExtendsTheFloor(t *testing.T) {
	p := platformWithSpec(t)
	var hits atomic.Int32
	clk := &fakeClock{now: t0}
	f := fetcherFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(profileJSON(t, p))
	}), FetcherConfig{Now: clk.Now})
	f.Platform(context.Background(), profileURL, false)
	clk.now = t0.Add(30 * time.Minute)
	f.Platform(context.Background(), profileURL, false)
	if hits.Load() != 1 {
		t.Fatalf("%d fetches; the origin's max-age applies above the floor", hits.Load())
	}
}

func TestFetcherFailuresAreTheRegisteredErrors(t *testing.T) {
	p := platformWithSpec(t)
	good := profileJSON(t, p)
	noServices := func() []byte { q := clone(t, p); q.UCP.Services = nil; return profileJSON(t, q) }()
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		max     int64
		code    string
		status  int
	}{
		{"not found", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }, 0, CodeProfileUnreachable, 424},
		{"server error", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }, 0, CodeProfileUnreachable, 424},
		{"redirect is never followed", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://example.com/elsewhere", http.StatusFound)
		}, 0, CodeProfileUnreachable, 424},
		{"permanent redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://example.com/elsewhere", http.StatusPermanentRedirect)
		}, 0, CodeProfileUnreachable, 424},
		{"not JSON", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) }, 0, CodeProfileMalformed, 422},
		{"violates the schema", func(w http.ResponseWriter, r *http.Request) { w.Write(noServices) }, 0, CodeProfileMalformed, 422},
		{"too large", func(w http.ResponseWriter, r *http.Request) { w.Write(good) }, 64, CodeProfileMalformed, 422},
		{"no ucp member", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) }, 0, CodeProfileMalformed, 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fetcherFor(t, tc.handler, FetcherConfig{MaxBody: tc.max})
			_, err := f.Platform(context.Background(), profileURL, false)
			mustReject(t, err, tc.code, tc.status)
		})
	}
	// An unreachable host.
	f := NewFetcher(FetcherConfig{Timeout: 2 * time.Second})
	_, err := f.Platform(context.Background(), "https://nonexistent.invalid/.well-known/ucp", false)
	mustReject(t, err, CodeProfileUnreachable, 424)
}

func TestProfileURLsAreValidatedBeforeAnythingIsFetched(t *testing.T) {
	f := NewFetcher(FetcherConfig{})
	for _, u := range []string{
		"", "http://platform.example/profile", "https://user:pw@platform.example/profile", "https://203.0.113.10/profile",
		"https://[::1]/profile", "https://localhost/profile", "ftp://platform.example/p", "platform.example/profile", "https:///profile",
	} {
		_, err := f.Platform(context.Background(), u, false)
		mustReject(t, err, CodeInvalidProfileURL, 400)
	}
}

func TestCacheIsBounded(t *testing.T) {
	p := platformWithSpec(t)
	var hits atomic.Int32
	f := fetcherFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write(profileJSON(t, p))
	}), FetcherConfig{Entries: 2})
	for _, u := range []string{"https://a.example.com/p", "https://b.example.com/p", "https://c.example.com/p"} {
		if _, err := f.Platform(context.Background(), u, false); err != nil {
			t.Fatal(err)
		}
	}
	if f.order.Len() != 2 || len(f.entries) != 2 {
		t.Fatalf("cache holds %d entries; the footprint is fixed at 2", f.order.Len())
	}
	hits.Store(0)
	f.Platform(context.Background(), "https://c.example.com/p", false) // newest: cached
	f.Platform(context.Background(), "https://a.example.com/p", false) // oldest: evicted, fetched again
	if hits.Load() != 1 {
		t.Fatalf("%d fetches; want 1", hits.Load())
	}
}

func TestSpecialUseAddressesAreRefused(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1": true, "::1": true, "10.1.2.3": true, "172.16.0.1": true, "172.31.255.255": true, "192.168.1.1": true,
		"169.254.169.254": true, "169.254.0.1": true, "fe80::1": true, "fc00::1": true, "fd12:3456::1": true,
		"0.0.0.0": true, "::": true, "100.64.0.1": true, "224.0.0.1": true, "255.255.255.255": true, "240.0.0.1": true,
		"192.0.2.1": true, "198.51.100.1": true, "203.0.113.5": true, "198.18.0.1": true, "2001:db8::1": true, "::ffff:10.0.0.1": true,
		"::ffff:169.254.169.254": true, "64:ff9b::a00:1": true,
		"8.8.8.8": false, "1.1.1.1": false, "93.184.216.34": false, "172.32.0.1": false, "2606:4700:4700::1111": false,
	} {
		if got := special(netip.MustParseAddr(addr)); got != want {
			t.Errorf("special(%s) = %v; want %v", addr, got, want)
		}
	}
}

func TestDialerRefusesTheResolvedAddress(t *testing.T) {
	f := NewFetcher(FetcherConfig{})
	for _, a := range []string{"127.0.0.1:443", "[::1]:443", "169.254.169.254:80", "10.0.0.5:443", "[fe80::1]:443"} {
		if err := f.control("tcp", a, nil); err == nil {
			t.Errorf("connect to %s was allowed", a)
		}
	}
	if err := f.control("tcp", "93.184.216.34:443", nil); err != nil {
		t.Errorf("a public address was refused: %v", err)
	}
	dev := NewFetcher(FetcherConfig{Loopback: true})
	if err := dev.control("tcp", "127.0.0.1:8443", nil); err != nil {
		t.Errorf("loopback with Loopback set: %v", err)
	}
	for _, a := range []string{"169.254.169.254:80", "10.0.0.5:443"} {
		if err := dev.control("tcp", a, nil); err == nil {
			t.Errorf("Loopback must not admit %s", a)
		}
	}
}

func TestHostClientIsStillNotAllowedToFollowRedirects(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	defer srv.Close()
	client := srv.Client()
	client.CheckRedirect = nil // the host's client would follow
	f := NewFetcher(FetcherConfig{Client: client})
	_, err := f.Platform(context.Background(), "https://example.com/x", false)
	mustReject(t, err, CodeProfileUnreachable, 424)
}
