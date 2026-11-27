package kraken

import (
	"context"
	"strings"
	"time"

	"github.com/hanzoai/commerce/payment/processor"
)

const (
	// DefaultAddressPoolSize is Kraken's maximum consecutive unused deposit addresses
	// per address family (e.g. 5 on Unified EVM, 5 on Solana SPL). Maintaining a full
	// 5-address rolling pool lets concurrent customers deposit to distinct fresh
	// wallets and rotates a brand-new replacement wallet in as each deposit settles.
	DefaultAddressPoolSize = 5

	poolCacheTTL = 45 * time.Second
)

type addressPool struct {
	addrs     []DepositAddress
	fetchedAt time.Time
	atCap     bool
	cursor    uint64
}

type poolTarget struct {
	chain string
	token string
}

func normalizePoolAddr(addr string) string {
	a := strings.TrimSpace(addr)
	if strings.HasPrefix(a, "0x") || strings.HasPrefix(a, "0X") {
		return strings.ToLower(a)
	}
	return a
}

// PoolFamilyKey maps a (chain, method) pair to the Kraken address pool it draws from.
// Kraken Unified EVM methods share the same 5 EVM deposit addresses across Base,
// Ethereum, Arbitrum, Optimism, Polygon and Avalanche; Solana SPL tokens share the
// same 5 Solana deposit addresses across USDC and USDT.
func PoolFamilyKey(chain, method string) string {
	m := strings.ToLower(strings.TrimSpace(method))
	c := strings.ToLower(strings.TrimSpace(chain))
	if strings.Contains(m, "unified") {
		return "evm-unified"
	}
	if c == "solana" || strings.Contains(m, "(spl)") || strings.Contains(m, "solana") {
		return "solana-spl"
	}
	if m != "" {
		return m
	}
	return c
}

func isTooManyAddressesErr(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "too many addresses")
}

func (p *Processor) poolForLocked(family string) *addressPool {
	if p.pools == nil {
		p.pools = make(map[string]*addressPool)
	}
	pl, ok := p.pools[family]
	if !ok {
		pl = &addressPool{}
		p.pools[family] = pl
	}
	return pl
}

func (p *Processor) isRetiredLocked(addr string) bool {
	if p.retired == nil {
		return false
	}
	return p.retired[normalizePoolAddr(addr)]
}

// IsRetired reports whether addr has been retired after a settled deposit.
func (p *Processor) IsRetired(addr string) bool {
	p.poolMu.Lock()
	defer p.poolMu.Unlock()
	return p.isRetiredLocked(addr)
}

func mergePoolAddresses(existing, incoming []DepositAddress, retired map[string]bool) []DepositAddress {
	seen := make(map[string]int, len(existing)+len(incoming))
	out := make([]DepositAddress, 0, len(existing)+len(incoming))
	add := func(a DepositAddress) {
		norm := normalizePoolAddr(a.Address)
		if norm == "" || (retired != nil && retired[norm]) {
			return
		}
		if idx, ok := seen[norm]; ok {
			out[idx] = a
			return
		}
		seen[norm] = len(out)
		out = append(out, a)
	}
	for _, a := range incoming {
		add(a)
	}
	for _, a := range existing {
		norm := normalizePoolAddr(a.Address)
		if _, already := seen[norm]; !already {
			add(a)
		}
	}
	return out
}

func hasAnyNewAddress(addrs []DepositAddress) bool {
	for _, a := range addrs {
		if a.New && strings.TrimSpace(a.Address) != "" {
			return true
		}
	}
	return false
}

// pickRollingLocked selects the next eligible address from addrs in round-robin order,
// advancing pl.cursor so sequential checkouts roll across all 5 addresses in the pool
// instead of repeatedly clustering on index 0.
func (p *Processor) pickRollingLocked(
	pl *addressPool,
	addrs []DepositAddress,
	free func(string) bool,
	fresh func(string) bool,
	allowSyntheticNonNew bool,
) (DepositAddress, bool) {
	n := len(addrs)
	if n == 0 {
		return DepositAddress{}, false
	}
	eligible := func(a DepositAddress, requireNew bool) bool {
		raw := strings.TrimSpace(a.Address)
		if raw == "" || p.isRetiredLocked(raw) {
			return false
		}
		if requireNew && !a.New {
			return false
		}
		if fresh != nil && !fresh(raw) {
			return false
		}
		if free != nil && !free(raw) {
			return false
		}
		return true
	}

	start := int(pl.cursor % uint64(n))
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		if eligible(addrs[idx], true) {
			pl.cursor = uint64((idx + 1) % n)
			return addrs[idx], true
		}
	}
	if allowSyntheticNonNew {
		for i := 0; i < n; i++ {
			idx := (start + i) % n
			if eligible(addrs[idx], false) {
				pl.cursor = uint64((idx + 1) % n)
				return addrs[idx], true
			}
		}
	}
	return DepositAddress{}, false
}

// acquirePoolAddress obtains a fresh, unoccupied deposit address from the 5-address
// rolling pool for (chain, token, asset, method), minting new addresses up to Kraken's
// 5-address cap and distributing checkouts round-robin across the pool.
func (p *Processor) acquirePoolAddress(ctx context.Context, chain, asset, method string) (DepositAddress, error) {
	free := processor.AddressFreeFromContext(ctx)
	fresh := processor.AddressFreshFromContext(ctx)
	family := PoolFamilyKey(chain, method)

	// Fast path: when all 5 unused addresses in this family are already minted and cached,
	// roll across the cached pool without burning Kraken private funding rate-limit quota.
	p.poolMu.Lock()
	pl := p.poolForLocked(family)
	if pl.atCap && len(pl.addrs) > 0 && time.Since(pl.fetchedAt) < poolCacheTTL {
		if chosen, ok := p.pickRollingLocked(pl, pl.addrs, free, fresh, false); ok {
			p.poolMu.Unlock()
			return chosen, nil
		}
	}
	p.poolMu.Unlock()

	queriedAll := false
	addrs, err := p.fetchDepositAddresses(ctx, asset, method, true)
	if isTooManyAddressesErr(err) {
		addrs, err = p.fetchDepositAddresses(ctx, asset, method, false)
		queriedAll = true
		if err == nil {
			p.poolMu.Lock()
			pl = p.poolForLocked(family)
			pl.atCap = true
			pl.addrs = mergePoolAddresses(nil, addrs, p.retired)
			pl.fetchedAt = time.Now()
			p.poolMu.Unlock()
		}
	} else if err == nil {
		p.poolMu.Lock()
		pl = p.poolForLocked(family)
		pl.addrs = mergePoolAddresses(pl.addrs, addrs, p.retired)
		if len(addrs) >= DefaultAddressPoolSize {
			pl.fetchedAt = time.Now()
		}
		p.poolMu.Unlock()
	}
	if err != nil {
		return DepositAddress{}, err
	}

	p.poolMu.Lock()
	pl = p.poolForLocked(family)
	candidates := addrs
	if queriedAll && len(pl.addrs) > 0 {
		candidates = pl.addrs
	}
	allowSynthetic := p.baseURL != defaultBaseURL && !hasAnyNewAddress(candidates) && !hasAnyNewAddress(pl.addrs)
	chosen, ok := p.pickRollingLocked(pl, candidates, free, fresh, allowSynthetic)
	p.poolMu.Unlock()
	if ok {
		return chosen, nil
	}

	// If new=true returned only the latest address and another buyer holds it while
	// other unused pool slots exist on Kraken, load the full pool with new=false.
	if !queriedAll {
		allAddrs, ferr := p.fetchDepositAddresses(ctx, asset, method, false)
		if ferr == nil {
			p.poolMu.Lock()
			pl = p.poolForLocked(family)
			pl.addrs = mergePoolAddresses(pl.addrs, allAddrs, p.retired)
			pl.fetchedAt = time.Now()
			allowSynthetic = p.baseURL != defaultBaseURL && !hasAnyNewAddress(pl.addrs)
			chosen, ok = p.pickRollingLocked(pl, pl.addrs, free, fresh, allowSynthetic)
			p.poolMu.Unlock()
			if ok {
				return chosen, nil
			}
		}
	}

	return DepositAddress{}, processor.NewPaymentError(processor.Kraken, "NO_FREE_ADDRESS",
		"kraken: no fresh deposit address in pool is free right now", nil)
}

// EnsurePool warms the Kraken deposit address pool for (chain, token) so up to target
// (default 5) unused deposit addresses exist and are ready for round-robin checkout.
func (p *Processor) EnsurePool(ctx context.Context, chain, token string, target int) (int, error) {
	if !p.IsAvailable(ctx) {
		return 0, processor.NewPaymentError(processor.Kraken, "NOT_CONFIGURED", "kraken processor not configured", nil)
	}
	if target <= 0 {
		target = DefaultAddressPoolSize
	}
	chain = strings.ToLower(strings.TrimSpace(chain))
	token = strings.ToLower(strings.TrimSpace(token))
	asset := krakenAsset(token, chain)
	method, err := p.resolveDepositMethod(ctx, chain, token, asset)
	if err != nil {
		return 0, err
	}
	family := PoolFamilyKey(chain, method)

	existing, err := p.fetchDepositAddresses(ctx, asset, method, false)
	if err != nil {
		return 0, err
	}
	p.poolMu.Lock()
	unused := 0
	for _, a := range existing {
		if a.New && strings.TrimSpace(a.Address) != "" && !p.isRetiredLocked(a.Address) {
			unused++
		}
	}
	pl := p.poolForLocked(family)
	pl.addrs = mergePoolAddresses(nil, existing, p.retired)
	pl.fetchedAt = time.Now()
	if unused >= target {
		pl.atCap = true
	}
	p.poolMu.Unlock()

	mintedAny := false
	for unused < target {
		if p.baseURL == defaultBaseURL && mintedAny {
			select {
			case <-ctx.Done():
				return unused, ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
		}
		created, merr := p.fetchDepositAddresses(ctx, asset, method, true)
		if isTooManyAddressesErr(merr) {
			p.poolMu.Lock()
			p.poolForLocked(family).atCap = true
			p.poolMu.Unlock()
			break
		}
		if merr != nil {
			return unused, merr
		}
		mintedAny = true
		p.poolMu.Lock()
		pl = p.poolForLocked(family)
		pl.addrs = mergePoolAddresses(pl.addrs, created, p.retired)
		p.poolMu.Unlock()
		unused++
	}

	if mintedAny {
		if refreshed, rerr := p.fetchDepositAddresses(ctx, asset, method, false); rerr == nil {
			p.poolMu.Lock()
			pl = p.poolForLocked(family)
			pl.addrs = mergePoolAddresses(pl.addrs, refreshed, p.retired)
			pl.fetchedAt = time.Now()
			count := 0
			for _, a := range pl.addrs {
				if a.New && !p.isRetiredLocked(a.Address) {
					count++
				}
			}
			unused = count
			pl.atCap = unused >= target
			p.poolMu.Unlock()
		}
	}
	return unused, nil
}

// RetireAndRotate permanently retires settledAddress from the active pool as soon as a
// deposit finishes and immediately requests a fresh replacement address (new=true) from
// Kraken. If Kraken's internal sweep has not yet released the 5-address slot, the family
// is queued for automatic background replenishment on subsequent watcher passes.
func (p *Processor) RetireAndRotate(ctx context.Context, chain, token, settledAddress string) error {
	norm := normalizePoolAddr(settledAddress)
	if norm == "" {
		return nil
	}
	chain = strings.ToLower(strings.TrimSpace(chain))
	token = strings.ToLower(strings.TrimSpace(token))

	p.poolMu.Lock()
	if p.retired == nil {
		p.retired = make(map[string]bool)
	}
	p.retired[norm] = true
	for _, pl := range p.pools {
		filtered := pl.addrs[:0]
		for _, a := range pl.addrs {
			if normalizePoolAddr(a.Address) != norm {
				filtered = append(filtered, a)
			}
		}
		pl.addrs = filtered
		pl.atCap = false
	}
	p.poolMu.Unlock()

	if !p.IsAvailable(ctx) || chain == "" {
		return nil
	}
	asset := krakenAsset(token, chain)
	method, err := p.resolveDepositMethod(ctx, chain, token, asset)
	if err != nil {
		return err
	}
	family := PoolFamilyKey(chain, method)

	created, err := p.fetchDepositAddresses(ctx, asset, method, true)
	if err != nil {
		p.poolMu.Lock()
		if p.pendingReplenish == nil {
			p.pendingReplenish = make(map[string]poolTarget)
		}
		p.pendingReplenish[family] = poolTarget{chain: chain, token: token}
		p.poolMu.Unlock()
		if isTooManyAddressesErr(err) {
			return nil
		}
		return err
	}

	p.poolMu.Lock()
	pl := p.poolForLocked(family)
	pl.addrs = mergePoolAddresses(pl.addrs, created, p.retired)
	pl.fetchedAt = time.Now()
	delete(p.pendingReplenish, family)
	p.poolMu.Unlock()

	if allAddrs, ferr := p.fetchDepositAddresses(ctx, asset, method, false); ferr == nil {
		p.poolMu.Lock()
		pl = p.poolForLocked(family)
		pl.addrs = mergePoolAddresses(pl.addrs, allAddrs, p.retired)
		pl.fetchedAt = time.Now()
		p.poolMu.Unlock()
	}
	return nil
}

// ReplenishPendingPools tops up any address family queued by RetireAndRotate back to
// DefaultAddressPoolSize unused deposit addresses once Kraken releases the settled slot.
func (p *Processor) ReplenishPendingPools(ctx context.Context) error {
	p.poolMu.Lock()
	if len(p.pendingReplenish) == 0 {
		p.poolMu.Unlock()
		return nil
	}
	targets := make(map[string]poolTarget, len(p.pendingReplenish))
	for k, v := range p.pendingReplenish {
		targets[k] = v
	}
	p.poolMu.Unlock()

	var firstErr error
	for family, t := range targets {
		unused, err := p.EnsurePool(ctx, t.chain, t.token, DefaultAddressPoolSize)
		if err != nil && firstErr == nil {
			firstErr = err
			continue
		}
		if unused >= DefaultAddressPoolSize {
			p.poolMu.Lock()
			delete(p.pendingReplenish, family)
			p.poolMu.Unlock()
		}
	}
	return firstErr
}
