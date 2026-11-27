package kraken

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hanzoai/commerce/payment/processor"
)

func newTestKrakenProcessor(t *testing.T, handler http.HandlerFunc) (*Processor, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	secret := base64.StdEncoding.EncodeToString([]byte("kraken-pool-test-secret-material-64-bytes-long-for-hmac-sha512!!"))
	p := NewProcessor(Config{
		APIKey:    "kraken-pool-key",
		APISecret: secret,
		APIURL:    srv.URL,
	})
	return p, srv
}

func TestEnsurePool_WarmsUpToFiveUnusedAddresses(t *testing.T) {
	var mu sync.Mutex
	solanaPool := []DepositAddress{
		{Address: "SolAddr1111111111111111111111111111111111111", New: true},
		{Address: "SolAddr2222222222222222222222222222222222222", New: true},
		{Address: "SolOldUsedAddr000000000000000000000000000000", New: false},
	}
	var newCalls int64

	p, _ := newTestKrakenProcessor(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/0/private/DepositMethods":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": []string{},
				"result": []DepositMethod{
					{Method: "USDC (SPL)", GenAddress: true, Minimum: "0.40"},
				},
			})
		case "/0/private/DepositAddresses":
			mu.Lock()
			defer mu.Unlock()
			if vals.Get("new") == "true" {
				atomic.AddInt64(&newCalls, 1)
				unused := 0
				for _, a := range solanaPool {
					if a.New {
						unused++
					}
				}
				if unused >= DefaultAddressPoolSize {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error":  []string{"EFunding:Too many addresses"},
						"result": []DepositAddress{},
					})
					return
				}
				minted := DepositAddress{
					Address: fmt.Sprintf("SolAddrFresh%d1111111111111111111111111111111", unused+1),
					New:     true,
				}
				solanaPool = append([]DepositAddress{minted}, solanaPool...)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":  []string{},
					"result": []DepositAddress{minted},
				})
				return
			}
			cp := make([]DepositAddress, len(solanaPool))
			copy(cp, solanaPool)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  []string{},
				"result": cp,
			})
		default:
			http.NotFound(w, r)
		}
	})

	unused, err := p.EnsurePool(context.Background(), "solana", "usdc", DefaultAddressPoolSize)
	if err != nil {
		t.Fatalf("EnsurePool: %v", err)
	}
	if unused != DefaultAddressPoolSize {
		t.Fatalf("unused = %d, want %d", unused, DefaultAddressPoolSize)
	}
	if got := atomic.LoadInt64(&newCalls); got != 3 {
		t.Fatalf("new=true calls = %d, want 3 (2 existing + 3 minted = 5)", got)
	}
	if min := p.CachedMinimumUSDCents("solana", "usdc"); min != 40 {
		t.Fatalf("CachedMinimumUSDCents(solana, usdc) = %d, want 40", min)
	}
}

func TestRollingPool_FiveAddressRoundRobinAndPostSettleRotation(t *testing.T) {
	var mu sync.Mutex
	var seq int
	var pool []DepositAddress
	for i := 1; i <= DefaultAddressPoolSize; i++ {
		seq = i
		pool = append(pool, DepositAddress{
			Address: fmt.Sprintf("0x%040x", i),
			New:     true,
		})
	}
	// Simulate Kraken sweep delay toggle: when true, new=true temporarily says Too many addresses
	// until Kraken flips the settled address New:false.
	sweepPending := true

	p, _ := newTestKrakenProcessor(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/0/private/DepositMethods":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": []string{},
				"result": []DepositMethod{
					{Method: "USDC - Base (Unified)", GenAddress: true, Minimum: "2.50"},
					{Method: "USDC - Arbitrum One (Unified)", GenAddress: true, Minimum: "2.50"},
					{Method: "USDC - Polygon (Unified)", GenAddress: true, Minimum: "2.00"},
				},
			})
		case "/0/private/DepositAddresses":
			mu.Lock()
			defer mu.Unlock()
			unused := 0
			for _, a := range pool {
				if a.New {
					unused++
				}
			}
			if vals.Get("new") == "true" {
				if unused >= DefaultAddressPoolSize || sweepPending {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error":  []string{"EFunding:Too many addresses"},
						"result": []DepositAddress{},
					})
					return
				}
				seq++
				minted := DepositAddress{Address: fmt.Sprintf("0x%040x", seq), New: true}
				pool = append([]DepositAddress{minted}, pool...)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":  []string{},
					"result": []DepositAddress{minted},
				})
				return
			}
			cp := make([]DepositAddress, len(pool))
			copy(cp, pool)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  []string{},
				"result": cp,
			})
		default:
			http.NotFound(w, r)
		}
	})

	held := make(map[string]bool)
	settled := make(map[string]bool)
	mkCtx := func() context.Context {
		ctx := processor.WithToken(context.Background(), "usdc")
		ctx = processor.WithAddressFree(ctx, func(a string) bool { return !held[a] })
		ctx = processor.WithAddressFresh(ctx, func(a string) bool { return !settled[a] })
		return ctx
	}

	// 5 checkouts across Unified EVM chains must roll across all 5 distinct addresses.
	chains := []string{"base", "arbitrum", "polygon", "base", "arbitrum"}
	got := make([]string, 0, 5)
	seen := make(map[string]bool)
	for i, ch := range chains {
		w, err := p.GenerateAddress(mkCtx(), fmt.Sprintf("org-%d/user", i+1), ch)
		if err != nil {
			t.Fatalf("GenerateAddress #%d (%s): %v", i+1, ch, err)
		}
		if seen[w.Address] {
			t.Fatalf("duplicate address %s handed out on checkout #%d", w.Address, i+1)
		}
		seen[w.Address] = true
		held[w.Address] = true
		got = append(got, w.Address)
	}

	// While all 5 are held, a 6th checkout must be refused cleanly (never reusing any of #1..#5).
	if w6, err := p.GenerateAddress(mkCtx(), "org-6/user", "base"); err == nil {
		t.Fatalf("6th concurrent checkout should have been refused while 5/5 held, got %s", w6.Address)
	}

	// Buyer 1's transaction settles on got[0]:
	//  - Even while Kraken's internal sweep hasn't flipped New:false yet (sweepPending=true),
	//    got[0] is immediately retired and queued for replenishment.
	delete(held, got[0])
	settled[got[0]] = true
	if err := p.RetireAndRotate(context.Background(), "base", "usdc", got[0]); err != nil {
		t.Fatalf("RetireAndRotate (sweep pending): %v", err)
	}
	if !p.IsRetired(got[0]) {
		t.Fatalf("settled address %s was not marked retired", got[0])
	}
	if wRetry, err := p.GenerateAddress(mkCtx(), "org-6/user", "base"); err == nil {
		t.Fatalf("settled address or held address handed out before replacement minted: %s", wRetry.Address)
	}

	// Kraken completes sweep on got[0] (New -> false); ReplenishPendingPools mints fresh 6th address!
	mu.Lock()
	for i := range pool {
		if pool[i].Address == got[0] {
			pool[i].New = false
		}
	}
	sweepPending = false
	mu.Unlock()

	if err := p.ReplenishPendingPools(context.Background()); err != nil {
		t.Fatalf("ReplenishPendingPools: %v", err)
	}

	w6, err := p.GenerateAddress(mkCtx(), "org-6/user", "arbitrum")
	if err != nil {
		t.Fatalf("GenerateAddress after rotation: %v", err)
	}
	wantAddr6 := fmt.Sprintf("0x%040x", 6)
	if w6.Address != wantAddr6 {
		t.Fatalf("rotated checkout got %s, want brand-new %s (never retired %s)", w6.Address, wantAddr6, got[0])
	}

	if minBase := p.CachedMinimumUSDCents("base", "usdc"); minBase != 250 {
		t.Fatalf("CachedMinimumUSDCents(base, usdc) = %d, want 250", minBase)
	}
	if minPoly := p.CachedMinimumUSDCents("polygon", "usdc"); minPoly != 200 {
		t.Fatalf("CachedMinimumUSDCents(polygon, usdc) = %d, want 200", minPoly)
	}
}

func TestInvalidNonce_RetriesOnceAutomatically(t *testing.T) {
	var attempts int64
	p, _ := newTestKrakenProcessor(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		n := atomic.AddInt64(&attempts, 1)
		if n == 1 {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  []string{"EAPI:Invalid nonce"},
				"result": []DepositMethod{},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": []string{},
			"result": []DepositMethod{
				{Method: "USDC - Base (Unified)", GenAddress: true, Minimum: "2.50"},
			},
		})
	})

	min := p.MinimumUSDCents(context.Background(), "base", "usdc")
	if min != 250 {
		t.Fatalf("MinimumUSDCents after nonce retry = %d, want 250", min)
	}
	if got := atomic.LoadInt64(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}
