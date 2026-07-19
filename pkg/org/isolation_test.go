// Copyright © 2026 Hanzo AI. MIT License.

package org

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hanzoai/commerce/models/organization"
)

// TestResolve_NeverServesAnotherTenant is the isolation gate. A cache keyed
// carelessly across orgs would hand org A's record — its namespace, its
// processor tokens, its secret key — to a request authenticated as org B. That
// is strictly worse than the memory growth the cache exists to remove, so it is
// pinned here: every resolve, warm or cold, returns the org that was asked for
// and nothing else.
func TestResolve_NeverServesAnotherTenant(t *testing.T) {
	setup(t)
	ctx := context.Background()

	names := []string{"alpha", "beta", "gamma", "delta"}

	// Warm every tenant so all subsequent resolves are cache HITS — the path
	// where a mis-keyed cache would cross tenants.
	for _, n := range names {
		if _, err := Resolve(ctx, n); err != nil {
			t.Fatalf("warm Resolve(%q): %v", n, err)
		}
	}

	// Interleave tenants repeatedly: a shared/overwritten entry shows up as a
	// name or namespace belonging to a different tenant.
	for i := 0; i < 100; i++ {
		for _, n := range names {
			o, err := Resolve(ctx, n)
			if err != nil {
				t.Fatalf("Resolve(%q): %v", n, err)
			}
			if o.Name != n {
				t.Fatalf("CROSS-TENANT: Resolve(%q) returned org named %q", n, o.Name)
			}
			// Namespace is the datastore scoping key — the value that decides
			// which tenant's rows every downstream query can reach.
			if ns := o.Namespace(); ns != n {
				t.Fatalf("CROSS-TENANT: Resolve(%q) returned namespace %q", n, ns)
			}
			if o.SecretKey == nil {
				t.Fatalf("Resolve(%q) returned an org with no secret key", n)
			}
		}
	}
}

// TestResolve_ConcurrentTenantsStayIsolated runs the interleave concurrently:
// the cache is shared process-wide, so isolation must hold under parallel
// resolves for different tenants, not just sequential ones.
func TestResolve_ConcurrentTenantsStayIsolated(t *testing.T) {
	setup(t)
	ctx := context.Background()

	names := []string{"acme", "globex", "initech", "umbrella"}
	for _, n := range names {
		if _, err := Resolve(ctx, n); err != nil {
			t.Fatalf("warm Resolve(%q): %v", n, err)
		}
	}

	var wg sync.WaitGroup
	for _, n := range names {
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(want string) {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					o, err := Resolve(ctx, want)
					if err != nil {
						t.Errorf("Resolve(%q): %v", want, err)
						return
					}
					if o.Name != want || o.Namespace() != want {
						t.Errorf("CROSS-TENANT: Resolve(%q) = name %q ns %q",
							want, o.Name, o.Namespace())
						return
					}
				}
			}(n)
		}
	}
	wg.Wait()
}

// TestResolve_ReturnsIndependentCopies pins the ownership contract. Callers
// mutate the resolved org per request — notably Live, which decides whether a
// charge reaches a sandbox or a real processor. If Resolve handed out the
// cached pointer, one request's Live would be visible to every concurrent
// request for that org, and a tampered field would poison the cache for its
// whole TTL.
func TestResolve_ReturnsIndependentCopies(t *testing.T) {
	setup(t)
	ctx := context.Background()

	first, err := Resolve(ctx, "acme")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	second, err := Resolve(ctx, "acme")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if first == second {
		t.Fatalf("Resolve returned the SAME pointer twice — callers would share mutable state")
	}

	liveBefore := first.Live

	// Mutate one caller's view the way the middleware does.
	first.Live = !liveBefore
	first.Name = "tampered"
	first.BillingEmail = "attacker@example.com"

	third, err := Resolve(ctx, "acme")
	if err != nil {
		t.Fatalf("Resolve after mutation: %v", err)
	}
	if third.Name != "acme" {
		t.Fatalf("cache poisoned: Resolve returned name %q after a caller tampered", third.Name)
	}
	if third.Live != liveBefore {
		t.Fatalf("cache poisoned: Live leaked across requests (%v -> %v)", liveBefore, third.Live)
	}
	if third.BillingEmail == "attacker@example.com" {
		t.Fatalf("cache poisoned: BillingEmail leaked across requests")
	}
	if second.Live != liveBefore {
		t.Fatalf("one caller's Live mutation was visible to another caller")
	}
}

// TestResolve_RejectsSecretLikeName pins the 2026-07-02 leak guard: a caller
// who presents a raw API key as their bearer must never cause the key to be
// persisted as an org name and tenant id. The guard runs BEFORE any cache read
// or store access, so a secret-shaped name can neither be provisioned nor
// retained in memory.
func TestResolve_RejectsSecretLikeName(t *testing.T) {
	cdb := setup(t)
	ctx := context.Background()

	for _, name := range []string{
		"sk-live-0123456789abcdef",
		"hk-0123456789abcdef",
		"SK-UPPERCASE-KEY",
		"  hk-leading-space",
	} {
		readsBefore := atomic.LoadInt64(&cdb.query)
		writesBefore := atomic.LoadInt64(&cdb.puts)

		o, err := Resolve(ctx, name)
		if !errors.Is(err, organization.ErrSecretLikeName) {
			t.Fatalf("Resolve(%q) err = %v, want ErrSecretLikeName", name, err)
		}
		if o != nil {
			t.Fatalf("Resolve(%q) returned an org %+v, want nil", name, o)
		}
		if got := atomic.LoadInt64(&cdb.query); got != readsBefore {
			t.Fatalf("Resolve(%q) read the store (%d -> %d) — guard must precede all store work",
				name, readsBefore, got)
		}
		if got := atomic.LoadInt64(&cdb.puts); got != writesBefore {
			t.Fatalf("Resolve(%q) WROTE the store (%d -> %d) — the secret would be persisted",
				name, writesBefore, got)
		}
		if _, cached := cache.Get(name); cached {
			t.Fatalf("Resolve(%q) put a secret-shaped name in the cache", name)
		}
	}
}

// TestResolve_HitBuildsAFreshOrgAndTouchesNoStore quantifies the steady state.
// A hit must hand back a caller-owned Organization — never a recycled pointer —
// and reach the store zero times. Before the cache, every request allocated that
// struct AND issued a query whose result the request held live while it waited;
// under pool starvation that is what filled the heap with tens of thousands of
// simultaneously-live orgs. (The per-call allocation COUNT is reported by
// BenchmarkResolveCacheHit, not asserted here — it is a cost, not a contract.)
func TestResolve_HitBuildsAFreshOrgAndTouchesNoStore(t *testing.T) {
	cdb := setup(t)
	ctx := context.Background()

	if _, err := Resolve(ctx, "hanzo"); err != nil {
		t.Fatalf("warm Resolve: %v", err)
	}
	readsWarm := atomic.LoadInt64(&cdb.query)
	writesWarm := atomic.LoadInt64(&cdb.puts)

	const n = 1000
	seen := make(map[*organization.Organization]struct{}, n)
	for i := 0; i < n; i++ {
		o, err := Resolve(ctx, "hanzo")
		if err != nil {
			t.Fatalf("Resolve %d: %v", i, err)
		}
		if _, dup := seen[o]; dup {
			t.Fatalf("Resolve returned a recycled pointer — copies must be per-caller")
		}
		seen[o] = struct{}{}
	}

	if got := atomic.LoadInt64(&cdb.query); got != readsWarm {
		t.Fatalf("%d cache hits read the store: %d -> %d", n, readsWarm, got)
	}
	if got := atomic.LoadInt64(&cdb.puts); got != writesWarm {
		t.Fatalf("%d cache hits wrote the store: %d -> %d", n, writesWarm, got)
	}
}

// BenchmarkResolveCacheHit reports the steady-state cost of the hot auth path.
// B/op is the per-request Organization copy; the store is not touched at all.
func BenchmarkResolveCacheHit(b *testing.B) {
	setup(b)
	ctx := context.Background()
	if _, err := Resolve(ctx, "hanzo"); err != nil {
		b.Fatalf("warm Resolve: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Resolve(ctx, "hanzo"); err != nil {
			b.Fatalf("Resolve: %v", err)
		}
	}
}
