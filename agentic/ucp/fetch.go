// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"container/list"
	"context"
	"crypto/tls"
	"encoding/json"
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

// Profiles resolves a platform's profile from its URL. refresh asks for a fresh
// copy rather than a cached one.
type Profiles interface {
	Platform(ctx context.Context, profile string, refresh bool) (*Profile, error)
}

// FetcherConfig configures a Fetcher. Every field has a default.
type FetcherConfig struct {
	// Client replaces the hardened client. A host that supplies one owns the
	// address checks the default client performs at connect time.
	Client *http.Client
	// TTL is the least time a profile is cached, whatever the origin's
	// Cache-Control says (overview, "Fetching", rule 4); default 60 seconds.
	TTL time.Duration
	// Entries bounds the cache so that discovery of unrecognised platforms has a
	// fixed footprint ("Fetching"); default 1024.
	Entries int
	// MaxBody bounds a profile's size; default 128 KiB, the least the
	// specification allows a verifier to set ("Fetching", rule 8).
	MaxBody int64
	// Timeout bounds one fetch, connect included; default 10 seconds.
	Timeout time.Duration
	// Loopback permits a loopback target, for a verifier running on the same
	// interface as the profile it fetches (local development; "Fetching", rule 7).
	Loopback bool
	// Now is the clock; it defaults to time.Now.
	Now func() time.Time
}

// Fetcher fetches and caches profiles under the rules of the overview
// ("Fetching"): HTTPS only, no redirects, no special-use addresses (checked on
// the connected address, so DNS rebinding does not get around it), a bounded
// body, a bounded cache with a TTL floor, and a forced refresh at most once per
// TTL per origin.
type Fetcher struct {
	cfg    FetcherConfig
	client *http.Client

	mu      sync.Mutex
	order   *list.List
	entries map[string]*list.Element
	forced  map[string]time.Time
}

type cached struct {
	url     string
	profile *Profile
	expires time.Time
}

// NewFetcher returns a Fetcher.
func NewFetcher(cfg FetcherConfig) *Fetcher {
	if cfg.TTL <= 0 {
		cfg.TTL = 60 * time.Second
	}
	if cfg.Entries <= 0 {
		cfg.Entries = 1024
	}
	if cfg.MaxBody <= 0 {
		cfg.MaxBody = 128 << 10
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	f := &Fetcher{cfg: cfg, order: list.New(), entries: map[string]*list.Element{}, forced: map[string]time.Time{}}
	if cfg.Client != nil {
		// The no-redirect rule is the specification's, not the host's choice.
		c := *cfg.Client
		c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		f.client = &c
	} else {
		f.client = guarded(cfg.Timeout, cfg.Loopback)
	}
	return f
}

func (f *Fetcher) now() time.Time {
	if f.cfg.Now != nil {
		return f.cfg.Now()
	}
	return time.Now()
}

// control refuses a connection to a special-use address, on the address the
// dialer actually resolved.
func (f *Fetcher) control(network, address string, c syscall.RawConn) error {
	return refuse(f.cfg.Loopback)(network, address, c)
}

func refuse(loopback bool) func(network, address string, _ syscall.RawConn) error {
	return func(_, address string, _ syscall.RawConn) error {
		ap, err := netip.ParseAddrPort(address)
		if err != nil {
			return err
		}
		if special(ap.Addr()) && !(loopback && ap.Addr().Unmap().IsLoopback()) {
			return fmt.Errorf("refusing to connect to special-use address %s", ap.Addr())
		}
		return nil
	}
}

// guarded returns an HTTP client for URLs that a counterparty supplied: it
// never follows a redirect, connects only to addresses that are not
// special-use (judged on the connected address, so DNS rebinding does not get
// around it), uses no proxy, and bounds the time of a request.
func guarded(timeout time.Duration, loopback bool) *http.Client {
	d := &net.Dialer{Timeout: timeout, Control: refuse(loopback)}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:         d.DialContext,
			TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
			TLSHandshakeTimeout: timeout,
			MaxIdleConns:        16,
			IdleConnTimeout:     30 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
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

// special reports whether a is a special-use address (RFC 6890): loopback,
// link-local including the cloud metadata address 169.254.169.254, private and
// the other reserved ranges. An IPv4-mapped IPv6 address is judged as IPv4.
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

// Platform returns the platform profile at profile. The profile is validated
// as a platform profile; entities that fail authority binding or the dev.ucp.*
// version rule are removed from it before it is cached.
func (f *Fetcher) Platform(ctx context.Context, profile string, refresh bool) (*Profile, error) {
	if err := checkURL(profile); err != nil {
		return nil, fail(CodeInvalidProfileURL, "profile URL: %v", err)
	}
	origin := originOf(profile)
	now := f.now()
	f.mu.Lock()
	if el, ok := f.entries[profile]; ok {
		c := el.Value.(*cached)
		allow := refresh && now.Sub(f.forced[origin]) >= f.cfg.TTL
		if now.Before(c.expires) && !allow {
			f.order.MoveToFront(el)
			f.mu.Unlock()
			return c.profile, nil
		}
		if allow {
			f.forced[origin] = now
		}
	} else if refresh {
		f.forced[origin] = now
	}
	f.mu.Unlock()

	p, ttl, err := f.fetch(ctx, profile)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if el, ok := f.entries[profile]; ok {
		el.Value = &cached{profile, p, f.now().Add(ttl)}
		f.order.MoveToFront(el)
		return p, nil
	}
	f.entries[profile] = f.order.PushFront(&cached{profile, p, f.now().Add(ttl)})
	for f.order.Len() > f.cfg.Entries {
		old := f.order.Back()
		f.order.Remove(old)
		delete(f.entries, old.Value.(*cached).url)
	}
	return p, nil
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, f.cfg.MaxBody+1))
	if err != nil {
		return nil, 0, fail(CodeProfileUnreachable, "unable to read profile: %v", err)
	}
	if int64(len(body)) > f.cfg.MaxBody {
		return nil, 0, fail(CodeProfileMalformed, "profile is larger than %d bytes", f.cfg.MaxBody)
	}
	var p Profile
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, 0, fail(CodeProfileMalformed, "profile is not valid JSON: %v", err)
	}
	if err := p.Wellformed(Platform); err != nil {
		return nil, 0, fail(CodeProfileMalformed, "%v", err)
	}
	p.Admit()
	ttl := f.cfg.TTL
	if age := maxAge(resp.Header.Get("Cache-Control")); age > ttl {
		ttl = age
	}
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
