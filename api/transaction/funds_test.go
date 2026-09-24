package transaction

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The guarantee is a property of CONCURRENT callers meeting one balance, so the
// tests race it. A sequential test of a lock proves a mutex locks, which nobody
// doubts, and would pass just as happily against the check-then-write these
// exist to rule out.

// Two callers spending the same source cannot both be inside the read-check-write
// at once. Without lockFunds both read Balance-Holds=100, both pass for 100, and
// both create — 200 held against a 100 balance.
func TestSameSourceIsSerialized(t *testing.T) {
	const callers = 32

	var inside int32
	var overlapped atomic.Bool
	var wg sync.WaitGroup

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := lockFunds("account", "acct_1", "USD", false)
			defer unlock()

			if atomic.AddInt32(&inside, 1) > 1 {
				overlapped.Store(true)
			}
			// Widen the window a real read-check-write occupies.
			time.Sleep(200 * time.Microsecond)
			atomic.AddInt32(&inside, -1)
		}()
	}
	wg.Wait()

	if overlapped.Load() {
		t.Fatal("two callers were inside one source's read-check-write at once — " +
			"the balance one of them read was already spent by the other")
	}
}

// Different sources must NOT serialize against each other, or one busy account
// throttles the whole ledger. This is the half a single global mutex gets wrong.
func TestDifferentSourcesRunConcurrently(t *testing.T) {
	// Distinct keys that land on distinct stripes. 256 stripes, so a handful of
	// well-separated ids collide only if the hash is broken — which is itself
	// worth knowing.
	keys := []string{"acct_a", "acct_b", "acct_c", "acct_d"}

	var mu sync.Mutex
	stripes := map[uintptr]bool{}
	for _, k := range keys {
		unlock := lockFunds("account", k, "USD", false)
		mu.Lock()
		stripes[stripeOf("account", k, "USD", false)] = true
		mu.Unlock()
		unlock()
	}
	if len(stripes) < 2 {
		t.Fatalf("%d distinct sources mapped to %d stripe(s) — the key is not discriminating",
			len(keys), len(stripes))
	}

	// And prove they genuinely overlap: each holds its own lock while the others
	// are held. A global lock would deadlock this or serialize it.
	var wg sync.WaitGroup
	start := make(chan struct{})
	var peak int32
	var cur int32
	for _, k := range keys {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			<-start
			unlock := lockFunds("account", k, "USD", false)
			defer unlock()
			n := atomic.AddInt32(&cur, 1)
			for {
				p := atomic.LoadInt32(&peak)
				if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
					break
				}
			}
			time.Sleep(2 * time.Millisecond)
			atomic.AddInt32(&cur, -1)
		}(k)
	}
	close(start)
	wg.Wait()

	if peak < 2 {
		t.Fatalf("peak concurrency across %d distinct sources was %d — distinct sources are serializing",
			len(keys), peak)
	}
}

// What must differ is the KEY, not the stripe. Two keys sharing a stripe only
// means they briefly serialize — harmless. Two distinct balances sharing a KEY
// would mean one caller's lock is protecting the wrong balance, which is the
// real failure, so these assert the key.
func TestDistinctBalancesGetDistinctKeys(t *testing.T) {
	base := fundsKey("account", "acct_1", "USD", false)

	for _, c := range []struct {
		name string
		key  string
	}{
		{"test money is a different ledger", fundsKey("account", "acct_1", "USD", true)},
		{"another currency is another balance", fundsKey("account", "acct_1", "EUR", false)},
		{"another source is another balance", fundsKey("account", "acct_2", "USD", false)},
		{"another source kind is another balance", fundsKey("user", "acct_1", "USD", false)},
	} {
		if c.key == base {
			t.Errorf("%s: shares a key with %q — one caller's lock would protect the other's balance", c.name, base)
		}
	}
}

// One key always maps to one stripe. This is the direction the guarantee runs;
// the reverse (distinct keys, distinct stripes) is explicitly NOT promised.
func TestOneKeyAlwaysOneStripe(t *testing.T) {
	first := stripeOf("account", "acct_1", "USD", false)
	for i := 0; i < 100; i++ {
		if got := stripeOf("account", "acct_1", "USD", false); got != first {
			t.Fatalf("the same balance mapped to stripe %d then %d — the lock is not stable", first, got)
		}
	}
}
