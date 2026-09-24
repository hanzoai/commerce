package checkout

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/commerce/models/organization"
)

func liveOrg(name string) *organization.Organization {
	o := &organization.Organization{}
	o.Name = name
	o.Live = true
	return o
}

// The whole point: a LIVE org row reaches the resolver, so the public org
// advertises production. This is the case the nil loader could not express.
func TestCachedOrgLoader_LiveOrgReachesResolver(t *testing.T) {
	t.Setenv("SQUARE_ENVIRONMENT", "production")
	t.Setenv("SQUARE_APPLICATION_ID", "sq0idp-PROD")
	t.Setenv("SQUARE_LOCATION_ID", "LOCPROD")

	l := NewCachedOrgLoader(func(_ context.Context, slug string) (*organization.Organization, error) {
		return liveOrg(slug), nil
	}, time.Minute, time.Second)

	ten, err := NewOrgResolver(l.Load).Resolve("pay.hanzo.ai")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ten.Square.Environment != "production" {
		t.Errorf("env = %q, want production for a LIVE org row", ten.Square.Environment)
	}
	if ten.Square.ApplicationID != "sq0idp-PROD" {
		t.Errorf("appID = %q, want the production app", ten.Square.ApplicationID)
	}
}

// An org that is NOT live stays sandbox even with a production deployment env —
// the org row is the authority, in both directions.
func TestCachedOrgLoader_NotLiveOrgStaysSandbox(t *testing.T) {
	t.Setenv("SQUARE_ENVIRONMENT", "production")
	t.Setenv("SQUARE_APPLICATION_ID", "sq0idp-PROD")
	t.Setenv("SQUARE_SANDBOX_APPLICATION_ID", "sandbox-sq0idb-TEST")
	t.Setenv("SQUARE_LOCATION_ID", "LOCPROD")

	l := NewCachedOrgLoader(func(_ context.Context, slug string) (*organization.Organization, error) {
		o := &organization.Organization{}
		o.Name = slug
		return o, nil // exists, not Live
	}, time.Minute, time.Second)

	ten, _ := NewOrgResolver(l.Load).Resolve("pay.hanzo.ai")
	if ten.Square.Environment != "sandbox" {
		t.Errorf("env = %q, want sandbox for a non-Live org", ten.Square.Environment)
	}
}

// A FAILED read must degrade to the synthetic org — i.e. sandbox. An outage can
// never promote a org onto production rails.
func TestCachedOrgLoader_ReadFailureFailsClosed(t *testing.T) {
	t.Setenv("SQUARE_ENVIRONMENT", "production")
	t.Setenv("SQUARE_APPLICATION_ID", "sq0idp-PROD")
	t.Setenv("SQUARE_LOCATION_ID", "LOCPROD")

	l := NewCachedOrgLoader(func(_ context.Context, _ string) (*organization.Organization, error) {
		return nil, errors.New("datastore down")
	}, time.Minute, time.Second)

	if _, ok := l.Load("hanzo"); ok {
		t.Fatal("a failed read reported a hit")
	}
	ten, _ := NewOrgResolver(l.Load).Resolve("pay.hanzo.ai")
	if ten.Square.Environment != "sandbox" {
		t.Errorf("env = %q, want sandbox when the org read fails", ten.Square.Environment)
	}
}

// The deadline is real: a read that hangs past the timeout is abandoned and
// reported as a miss, so one slow datastore cannot hold the endpoint open.
func TestCachedOrgLoader_SlowReadIsBounded(t *testing.T) {
	l := NewCachedOrgLoader(func(ctx context.Context, _ string) (*organization.Organization, error) {
		<-ctx.Done() // honor the deadline, as the contract requires
		return nil, ctx.Err()
	}, time.Minute, 40*time.Millisecond)

	start := time.Now()
	if _, ok := l.Load("hanzo"); ok {
		t.Fatal("a timed-out read reported a hit")
	}
	if el := time.Since(start); el > time.Second {
		t.Errorf("Load took %v — the deadline did not bound it", el)
	}
}

// A hit costs no I/O. This is what keeps the SPA-boot storm off the datastore.
func TestCachedOrgLoader_CachesHitsAndMisses(t *testing.T) {
	var mu sync.Mutex
	calls := map[string]int{}
	l := NewCachedOrgLoader(func(_ context.Context, slug string) (*organization.Organization, error) {
		mu.Lock()
		calls[slug]++
		mu.Unlock()
		if slug == "hanzo" {
			return liveOrg(slug), nil
		}
		return nil, nil // a real miss: no row
	}, time.Minute, time.Second)

	for i := 0; i < 5; i++ {
		l.Load("hanzo")
		l.Load("nobody")
	}

	mu.Lock()
	defer mu.Unlock()
	if calls["hanzo"] != 1 {
		t.Errorf("hit read %d times, want 1", calls["hanzo"])
	}
	// Negative caching matters as much: a brand with no row is the common case.
	if calls["nobody"] != 1 {
		t.Errorf("miss read %d times, want 1 (misses must cache too)", calls["nobody"])
	}
}

// Expiry: flipping an org Live takes effect within the TTL, with no restart.
func TestCachedOrgLoader_ExpiresSoAFlipLands(t *testing.T) {
	live := false
	l := NewCachedOrgLoader(func(_ context.Context, slug string) (*organization.Organization, error) {
		o := &organization.Organization{}
		o.Name = slug
		o.Live = live
		return o, nil
	}, 30*time.Millisecond, time.Second)

	if o, _ := l.Load("hanzo"); o.Live {
		t.Fatal("org read Live before the flip")
	}
	live = true
	if o, _ := l.Load("hanzo"); o.Live {
		t.Fatal("cache did not hold within its TTL")
	}
	time.Sleep(45 * time.Millisecond)
	if o, _ := l.Load("hanzo"); !o.Live {
		t.Error("the flip never landed after the TTL expired")
	}
}

// A nil loader is a miss, not a panic — wiring one in can't crash the endpoint.
func TestCachedOrgLoader_NilIsSafe(t *testing.T) {
	var c *CachedOrgLoader
	if _, ok := c.Load("hanzo"); ok {
		t.Error("nil loader reported a hit")
	}
	if _, ok := NewCachedOrgLoader(nil, 0, 0).Load("hanzo"); ok {
		t.Error("nil read reported a hit")
	}
}
