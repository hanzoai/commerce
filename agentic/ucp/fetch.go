// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// The overview's rules for fetching a counterparty's document ("Fetching").
const (
	// minTTL is the floor of the cache lifetime of a profile (rule 4).
	minTTL = 60 * time.Second
	// maxBody bounds a profile's size: the least a verifier may set (rule 8).
	maxBody = 128 << 10
)

// Profiles resolves a platform's profile from its URL; refresh asks for a fresh
// copy rather than a cached one.
type Profiles interface {
	Platform(ctx context.Context, profile string, refresh bool) (*Profile, error)
}

// FetcherConfig configures a Fetcher. Every field is required.
type FetcherConfig struct {
	// Client fetches profiles; NewClient builds one. Its Timeout bounds a fetch.
	Client *http.Client
	// Entries bounds the cache, so discovery of unrecognised platforms has a
	// fixed footprint.
	Entries int
	// MaxTTL is the longest a profile is cached whatever its Cache-Control says,
	// and not less than the 60 second floor: a revoked key stays usable until the
	// profile that published it expires.
	MaxTTL time.Duration
	// Concurrent bounds the fetches in flight; a request beyond it is answered
	// 503 ("Fetching", asynchronous discovery).
	Concurrent int
}

// Fetcher fetches and caches profiles under the overview's rules ("Fetching"):
// HTTPS only, no redirects, a bounded body, a bounded cache with a TTL floor and
// ceiling, a bounded number of fetches with concurrent requests for one profile
// sharing one, and a forced refresh at most once per TTL per origin.
type Fetcher struct {
	cfg    FetcherConfig
	client *http.Client
	now    func() time.Time
	sem    chan struct{}

	mu      sync.Mutex
	order   *list.List
	entries map[string]*list.Element
	forced  map[string]time.Time
	flights map[string]*flight
}

type cached struct {
	url     string
	profile *Profile
	expires time.Time
}

// flight is a fetch in progress that other requests for the same profile wait on.
type flight struct {
	done    chan struct{}
	profile *Profile
	err     error
}

// NewFetcher returns a Fetcher.
func NewFetcher(cfg FetcherConfig) (*Fetcher, error) {
	switch {
	case cfg.Client == nil || cfg.Client.Timeout <= 0:
		return nil, errors.New("ucp: FetcherConfig.Client needs a Timeout")
	case cfg.Entries <= 0 || cfg.Concurrent <= 0:
		return nil, errors.New("ucp: FetcherConfig needs Entries and Concurrent")
	case cfg.MaxTTL < minTTL:
		return nil, errors.New("ucp: FetcherConfig.MaxTTL is less than the 60 second floor")
	}
	return &Fetcher{
		cfg: cfg, client: withoutRedirects(cfg.Client), now: time.Now, sem: make(chan struct{}, cfg.Concurrent),
		order: list.New(), entries: map[string]*list.Element{}, forced: map[string]time.Time{}, flights: map[string]*flight{},
	}, nil
}

// withoutRedirects copies a client so that it never follows a redirect (rule 2).
func withoutRedirects(c *http.Client) *http.Client {
	cc := *c
	cc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &cc
}

// NewClient returns an HTTP client for URLs a counterparty supplied: it connects
// only to addresses that are not special-use, judged on the connected address so
// that DNS rebinding does not get around it (rule 7), and uses no proxy.
func NewClient(timeout time.Duration) *http.Client {
	d := &net.Dialer{Timeout: timeout, Control: refuse}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:            d.DialContext,
			TLSHandshakeTimeout:    timeout,
			MaxResponseHeaderBytes: maxBody,
			DisableKeepAlives:      true,
		},
	}
}

// refuse is a dialer Control that rejects a connection to a special-use address.
func refuse(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return err
	}
	if special(ap.Addr()) {
		return fmt.Errorf("refusing to connect to special-use address %s", ap.Addr())
	}
	return nil
}

var specialPrefixes = func() []netip.Prefix {
	var out []netip.Prefix
	for _, s := range []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12",
		"192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16", "198.18.0.0/15",
		"198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
		"::/128", "::1/128", "64:ff9b::/96", "100::/64", "2001::/23", "2001:db8::/32", "2002::/16",
		"fc00::/7", "fe80::/10", "fec0::/10", "ff00::/8",
	} {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()

// special reports whether a is a special-use address (RFC 6890). An IPv4-mapped
// IPv6 address is judged as IPv4.
func special(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsMulticast() || a.IsUnspecified() {
		return true
	}
	for _, p := range specialPrefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// Platform returns the platform profile at profile, validated as a platform
// profile with the entities that fail authority binding or the dev.ucp.*
// version rule removed. Requests for a profile already being fetched wait for
// that fetch; one beyond Concurrent is refused with 503.
func (f *Fetcher) Platform(ctx context.Context, profile string, refresh bool) (*Profile, error) {
	if err := checkURL(profile); err != nil {
		return nil, fail(CodeInvalidProfileURL, "profile URL: %v", err)
	}
	origin := originOf(profile)
	now := f.now()
	f.mu.Lock()
	force := refresh && f.allowForce(origin, now)
	if el, ok := f.entries[profile]; ok {
		c := el.Value.(*cached)
		if now.Before(c.expires) && !force {
			f.order.MoveToFront(el)
			f.mu.Unlock()
			return c.profile, nil
		}
	}
	if fl, ok := f.flights[profile]; ok {
		f.mu.Unlock()
		select {
		case <-fl.done:
			return fl.profile, fl.err
		case <-ctx.Done():
			return nil, fail(CodeProfileUnreachable, "profile fetch canceled: %v", ctx.Err())
		}
	}
	select {
	case f.sem <- struct{}{}:
	default:
		f.mu.Unlock()
		return nil, status(http.StatusServiceUnavailable, "unavailable", "profile discovery is saturated")
	}
	fl := &flight{done: make(chan struct{})}
	f.flights[profile] = fl
	f.mu.Unlock()

	// The fetch is shared, so it is not bound to the leader's request.
	p, ttl, err := f.fetch(context.WithoutCancel(ctx), profile)

	f.mu.Lock()
	delete(f.flights, profile)
	<-f.sem
	if err == nil {
		f.store(profile, p, ttl)
	}
	fl.profile, fl.err = p, err
	f.mu.Unlock()
	close(fl.done)
	return p, err
}

// allowForce reports whether a forced refresh of origin may happen now, and
// records it: at most one per TTL per origin (rule 6). The record is bounded by
// the cache size.
func (f *Fetcher) allowForce(origin string, now time.Time) bool {
	if last, ok := f.forced[origin]; ok && now.Sub(last) < minTTL {
		return false
	}
	if len(f.forced) >= f.cfg.Entries {
		for o, t := range f.forced {
			if now.Sub(t) >= minTTL {
				delete(f.forced, o)
			}
		}
		if len(f.forced) >= f.cfg.Entries {
			return false
		}
	}
	f.forced[origin] = now
	return true
}

// store caches a profile, evicting the least recently used beyond Entries.
func (f *Fetcher) store(profile string, p *Profile, ttl time.Duration) {
	expires := f.now().Add(ttl)
	if el, ok := f.entries[profile]; ok {
		el.Value = &cached{profile, p, expires}
		f.order.MoveToFront(el)
		return
	}
	f.entries[profile] = f.order.PushFront(&cached{profile, p, expires})
	for f.order.Len() > f.cfg.Entries {
		old := f.order.Back()
		f.order.Remove(old)
		delete(f.entries, old.Value.(*cached).url)
	}
}

func originOf(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return s
	}
	return u.Scheme + "://" + strings.ToLower(u.Host)
}

func (f *Fetcher) fetch(ctx context.Context, profile string) (*Profile, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, profile, nil)
	if err != nil {
		return nil, 0, fail(CodeInvalidProfileURL, "profile URL: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, 0, fail(CodeProfileUnreachable, "unable to fetch profile: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, 0, fail(CodeProfileUnreachable, "unable to fetch profile: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, 0, fail(CodeProfileUnreachable, "unable to read profile: %v", err)
	}
	if len(body) > maxBody {
		return nil, 0, fail(CodeProfileMalformed, "profile is larger than %d bytes", maxBody)
	}
	var p Profile
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, 0, fail(CodeProfileMalformed, "profile is not valid JSON: %v", err)
	}
	if err := p.Wellformed(Platform); err != nil {
		return nil, 0, fail(CodeProfileMalformed, "%v", err)
	}
	p.Admit()
	ttl := min(max(minTTL, maxAge(resp.Header.Get("Cache-Control"))), f.cfg.MaxTTL)
	return &p, ttl, nil
}

// maxAge reads max-age from a Cache-Control value.
func maxAge(h string) time.Duration {
	for _, d := range strings.Split(h, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(d), "=")
		if ok && strings.EqualFold(k, "max-age") {
			if n, err := strconv.Atoi(strings.Trim(v, `"`)); err == nil && n > 0 {
				return time.Duration(n) * time.Second
			}
		}
	}
	return 0
}

// AgentKeys resolves the keys of a request's signer by the default UCP lookup:
// the keys[] of the profile the UCP-Agent header names.
type AgentKeys struct{ Profiles Profiles }

// Keys implements Keys.
func (a AgentKeys) Keys(ctx context.Context, r *http.Request, refresh bool) (string, []Key, error) {
	u, e := ParseAgent(r.Header.Get(HeaderAgent))
	if e != nil {
		return "", nil, e
	}
	p, err := a.Profiles.Platform(ctx, u, refresh)
	if err != nil {
		return "", nil, err
	}
	return u, p.Keys, nil
}
