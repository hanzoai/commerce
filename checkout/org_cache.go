package checkout

import (
	"context"
	"sync"
	"time"

	"github.com/hanzoai/commerce/models/organization"
)

// CachedOrgLoader wraps a slow, fallible org read in the two properties
// OrgLoader's contract REQUIRES of any real loader: a cache, and a deadline.
//
// Resolve runs on GET /v1/commerce/org — public, unauthenticated, and hit by
// the pay SPA on every boot. A naive per-request org query under an unbounded
// context blocks and exhausts the DB pool; that is the 1.42.44 regression, and
// it is why the loader defaulted to nil. nil is safe but it is not free: with no
// loader every host resolves to a SYNTHETIC org, a synthetic org is never Live,
// and the public org JSON therefore advertises the SANDBOX Square application
// forever — even after the org record is flipped Live (measured 2026-07-30:
// POST /v1/billing/mode returned {"live":true} and the org kept serving
// sandbox-sq0idb-…, so the card iframe tokenized against sandbox while the charge
// path used the live org — a nonce the production account cannot charge).
//
// So the org row must be read, and reading it must not be able to hurt the
// endpoint. This type is that bargain, and nothing else:
//
//   - ttl bounds staleness. A hit costs a mutex, never I/O. Flipping an org
//     live takes effect within ttl with no restart.
//   - timeout bounds the miss. The read runs under its own deadline, so a slow
//     or wedged datastore costs one request that many rather than a pool.
//   - a failed or timed-out read returns (nil,false), which degrades to exactly
//     the synthetic-org behavior that shipped before — SANDBOX. The fallback on
//     every error path is the fail-closed one, so an outage can never promote a
//     org onto production rails.
//
// Negative results are cached too, and deliberately: a brand with no org row is
// the common case for the non-default brands, and re-querying for a row that
// does not exist on every SPA boot is the same stampede in a different costume.
type CachedOrgLoader struct {
	read    func(ctx context.Context, slug string) (*organization.Organization, error)
	ttl     time.Duration
	timeout time.Duration

	mu      sync.Mutex
	entries map[string]orgEntry
}

type orgEntry struct {
	org *organization.Organization // nil == a cached miss
	at  time.Time
}

// NewCachedOrgLoader builds a loader over read. Zero or negative ttl/timeout
// take the defaults (60s / 2s). read must be READ-ONLY and must honor its ctx.
func NewCachedOrgLoader(
	read func(ctx context.Context, slug string) (*organization.Organization, error),
	ttl, timeout time.Duration,
) *CachedOrgLoader {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &CachedOrgLoader{
		read:    read,
		ttl:     ttl,
		timeout: timeout,
		entries: map[string]orgEntry{},
	}
}

// Load is the OrgLoader. A nil receiver or nil read reports a miss, so wiring
// one in is never a nil-deref hazard.
func (c *CachedOrgLoader) Load(slug string) (*organization.Organization, bool) {
	if c == nil || c.read == nil {
		return nil, false
	}

	now := time.Now()
	c.mu.Lock()
	e, ok := c.entries[slug]
	c.mu.Unlock()
	if ok && now.Sub(e.at) < c.ttl {
		return e.org, e.org != nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	org, err := c.read(ctx, slug)
	if err != nil {
		org = nil // a failed read is a MISS, never a live org
	}

	c.mu.Lock()
	c.entries[slug] = orgEntry{org: org, at: now}
	c.mu.Unlock()

	return org, org != nil
}
