// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The httptest certificate is valid for example.com; the client dials the test
// server whatever the host, so a profile URL can carry a real host name.
const profileURL = "https://example.com/.well-known/ucp"

// testClient is a client that reaches the test server for every host and trusts
// its certificate.
func testClient(srv *httptest.Server) *http.Client {
	tr := srv.Client().Transport.(*http.Transport).Clone()
	tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}
	return &http.Client{Transport: tr, Timeout: 10 * time.Second}
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

// fetcherFor serves h over TLS and returns a Fetcher with the given limits
// (Entries 100, MaxTTL a day and Concurrent 8 unless the test sets them), reading
// time from clk when it is not nil.
func fetcherFor(t *testing.T, h http.Handler, cfg FetcherConfig, clk *fakeClock) *Fetcher {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	cfg.Client = testClient(srv)
	return newFetcher(t, cfg, clk)
}

func newFetcher(t *testing.T, cfg FetcherConfig, clk *fakeClock) *Fetcher {
	t.Helper()
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: time.Second}
	}
	if cfg.Entries == 0 {
		cfg.Entries = 100
	}
	if cfg.MaxTTL == 0 {
		cfg.MaxTTL = 24 * time.Hour
	}
	if cfg.Concurrent == 0 {
		cfg.Concurrent = 8
	}
	f, err := NewFetcher(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if clk != nil {
		f.now = clk.Now
	}
	return f
}

func profileJSON(t *testing.T, p *Profile) []byte {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

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
	}), FetcherConfig{}, clk)

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
	}), FetcherConfig{}, clk)
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
	}), FetcherConfig{}, clk)
	f.Platform(context.Background(), profileURL, false)
	clk.now = t0.Add(30 * time.Minute)
	f.Platform(context.Background(), profileURL, false)
	if hits.Load() != 1 {
		t.Fatalf("%d fetches; the origin's max-age applies above the floor", hits.Load())
	}
}

func TestFetcherFailuresAreTheRegisteredErrors(t *testing.T) {
	p := platformWithSpec(t)
	noServices := func() []byte { q := clone(t, p); q.UCP.Services = nil; return profileJSON(t, q) }()
	large := []byte(`{"pad":"` + strings.Repeat("a", maxBody) + `"}`)
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		code    string
		status  int
	}{
		{"not found", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }, CodeProfileUnreachable, 424},
		{"server error", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }, CodeProfileUnreachable, 424},
		{"redirect is never followed", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://example.com/elsewhere", http.StatusFound)
		}, CodeProfileUnreachable, 424},
		{"permanent redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://example.com/elsewhere", http.StatusPermanentRedirect)
		}, CodeProfileUnreachable, 424},
		{"not JSON", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) }, CodeProfileMalformed, 422},
		{"violates the schema", func(w http.ResponseWriter, r *http.Request) { w.Write(noServices) }, CodeProfileMalformed, 422},
		{"too large", func(w http.ResponseWriter, r *http.Request) { w.Write(large) }, CodeProfileMalformed, 422},
		{"no ucp member", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) }, CodeProfileMalformed, 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fetcherFor(t, tc.handler, FetcherConfig{}, nil)
			_, err := f.Platform(context.Background(), profileURL, false)
			mustReject(t, err, tc.code, tc.status)
		})
	}
	// An unreachable host.
	f := newFetcher(t, FetcherConfig{Client: NewClient(2 * time.Second)}, nil)
	_, err := f.Platform(context.Background(), "https://nonexistent.invalid/.well-known/ucp", false)
	mustReject(t, err, CodeProfileUnreachable, 424)
}

func TestProfileURLsAreValidatedBeforeAnythingIsFetched(t *testing.T) {
	f := newFetcher(t, FetcherConfig{}, nil)
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
	}), FetcherConfig{Entries: 2}, nil)
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

// The default client refuses a special-use address at connect time, so a profile
// or webhook URL that resolves to one is never reached.
func TestClientRefusesTheResolvedAddress(t *testing.T) {
	dial := NewClient(time.Second).Transport.(*http.Transport).DialContext
	for _, a := range []string{"127.0.0.1:9", "[::1]:9", "169.254.169.254:80", "10.0.0.5:443", "[fe80::1]:443"} {
		c, err := dial(context.Background(), "tcp", a)
		if err == nil {
			c.Close()
			t.Errorf("connect to %s was allowed", a)
		} else if !strings.Contains(err.Error(), "special-use") {
			t.Errorf("%s: %v", a, err)
		}
	}
}

func TestHostClientIsStillNotAllowedToFollowRedirects(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	defer srv.Close()
	client := testClient(srv)
	client.CheckRedirect = nil // the host's client would follow
	f := newFetcher(t, FetcherConfig{Client: client}, nil)
	_, err := f.Platform(context.Background(), "https://example.com/x", false)
	mustReject(t, err, CodeProfileUnreachable, 424)
}

func TestFetcherNeedsItsLimitsStated(t *testing.T) {
	ok := FetcherConfig{Client: &http.Client{Timeout: time.Second}, Entries: 1, MaxTTL: time.Hour, Concurrent: 1}
	for name, mutate := range map[string]func(*FetcherConfig){
		"no client":                  func(c *FetcherConfig) { c.Client = nil },
		"a client without a timeout": func(c *FetcherConfig) { c.Client = &http.Client{} },
		"no cache size":              func(c *FetcherConfig) { c.Entries = 0 },
		"no concurrency cap":         func(c *FetcherConfig) { c.Concurrent = 0 },
		"no ceiling":                 func(c *FetcherConfig) { c.MaxTTL = 0 },
		"ceiling under the floor":    func(c *FetcherConfig) { c.MaxTTL = 30 * time.Second },
	} {
		cfg := ok
		mutate(&cfg)
		if _, err := NewFetcher(cfg); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := NewFetcher(ok); err != nil {
		t.Fatal(err)
	}
}

// A profile cannot outlive the ceiling whatever its Cache-Control says: a
// revoked key must stop verifying.
func TestCacheLifetimeHasACeiling(t *testing.T) {
	p := platformWithSpec(t)
	var hits atomic.Int32
	clk := &fakeClock{now: t0}
	f := fetcherFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Write(profileJSON(t, p))
	}), FetcherConfig{MaxTTL: time.Hour}, clk)
	f.Platform(context.Background(), profileURL, false)
	clk.now = t0.Add(59 * time.Minute)
	f.Platform(context.Background(), profileURL, false)
	if hits.Load() != 1 {
		t.Fatalf("%d fetches inside the ceiling", hits.Load())
	}
	clk.now = t0.Add(61 * time.Minute)
	f.Platform(context.Background(), profileURL, false)
	if hits.Load() != 2 {
		t.Fatalf("%d fetches; a year-long max-age must not outlive the ceiling", hits.Load())
	}
}

// Concurrent requests for one profile share one fetch.
func TestConcurrentFetchesOfOneProfileAreCoalesced(t *testing.T) {
	p := platformWithSpec(t)
	var hits atomic.Int32
	release := make(chan struct{})
	f := fetcherFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-release
		w.Write(profileJSON(t, p))
	}), FetcherConfig{}, nil)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.Platform(context.Background(), profileURL, false)
			errs <- err
		}()
	}
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("%d fetches for one profile", hits.Load())
	}
}

// Fetches of different profiles are capped; past the cap a request is refused
// with 503, so discovery of unrecognised platforms has a fixed footprint.
func TestProfileFetchesAreCapped(t *testing.T) {
	p := platformWithSpec(t)
	var running, peak atomic.Int32
	release := make(chan struct{})
	f := fetcherFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := running.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		<-release
		running.Add(-1)
		w.Write(profileJSON(t, p))
	}), FetcherConfig{Concurrent: 2}, nil)
	results := make(chan error, 6)
	for i := 0; i < 6; i++ {
		go func(i int) {
			_, err := f.Platform(context.Background(), fmt.Sprintf("https://p%d.example.com/profile", i), false)
			results <- err
		}(i)
	}
	for i := 0; i < 4; i++ { // the four beyond the cap answer at once
		var e *Error
		if err := <-results; !errors.As(err, &e) || e.Status != 503 {
			t.Fatalf("beyond the cap: %v", err)
		}
	}
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if peak.Load() > 2 {
		t.Fatalf("peak %d concurrent fetches; the cap is 2", peak.Load())
	}
}

// The record of forced refreshes does not grow with every origin seen.
func TestForcedRefreshRecordIsPruned(t *testing.T) {
	p := platformWithSpec(t)
	clk := &fakeClock{now: t0}
	f := fetcherFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(profileJSON(t, p)) }),
		FetcherConfig{Entries: 4}, clk)
	for i := 0; i < 50; i++ {
		f.Platform(context.Background(), fmt.Sprintf("https://o%d.example.com/p", i), true)
		clk.now = clk.now.Add(30 * time.Second)
	}
	f.mu.Lock()
	n := len(f.forced)
	f.mu.Unlock()
	if n > 4 {
		t.Fatalf("%d forced-refresh records for 4 cache entries", n)
	}
}

// Response headers are bounded.
func TestResponseHeadersAreBounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Big", strings.Repeat("a", 4*maxBody))
	}))
	defer srv.Close()
	tr := NewClient(5 * time.Second).Transport.(*http.Transport).Clone()
	tr.DialContext = nil // the test server is on loopback
	if resp, err := (&http.Client{Transport: tr}).Get(srv.URL); err == nil {
		resp.Body.Close()
		t.Fatal("a response header four times the profile bound was accepted")
	}
}
