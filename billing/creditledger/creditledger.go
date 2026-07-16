// Package creditledger is the injection seam for the ONE double-entry credit
// ledger commerce's billing writes to.
//
// Commerce runs BOTH standalone and EMBEDDED in the cloud binary. When embedded,
// the AI spend-gate reads cloud's native ledger (a per-org finance ledger);
// commerce is a SEPARATE Go module and must NOT import cloud. So the host injects
// a ledger-backed CreditLedger through commerce.EmbedConfig.Ledger, and
// commerce's credit + balance handlers route to it — a credit then lands in the
// SAME per-org account the gate reads (one ledger, no split). When no ledger is
// injected (standalone dev/test), commerce falls back to its own datastore.
//
// The contract is compiler-enforced: cloud implements this interface (a ~40-line
// adapter over its finance ledger) and sets EmbedConfig.Ledger; commerce calls it.
package creditledger

import (
	"context"
	"sync"
	"time"
)

// CreditInput is one org-keyed, idempotent credit grant. The ledger is
// multi-currency, so Currency is explicit. ExpiresAt nil = no expiry.
type CreditInput struct {
	Org            string
	Currency       string
	Reason         string
	Tag            string
	IdempotencyKey string
	AmountCents    int64
	ExpiresAt      *time.Time
}

// CreditLedger is the ONE way credit enters an org ledger when commerce is
// embedded. Both methods are org-keyed and MUST resolve the SAME per-org account,
// so a Credit is immediately visible to Balance and to the cloud AI gate that
// reads the same ledger account.
type CreditLedger interface {
	// Credit appends a BALANCED double-entry credit to the org's account and
	// returns the posting/tx id and the account's new available balance.
	// Idempotent on IdempotencyKey: the same key credits AT MOST once and returns
	// the original posting's id + balance (never a second grant).
	Credit(ctx context.Context, in CreditInput) (txID string, balanceCents int64, err error)
	// Balance returns the org account's available balance in cents for currency.
	Balance(ctx context.Context, org, currency string) (availableCents int64, err error)
}

var (
	mu       sync.RWMutex
	injected CreditLedger
)

// Set installs the host-injected ledger. commerce.Embed calls it from
// EmbedConfig.Ledger; passing nil clears it (→ commerce falls back to its own
// datastore). Process-wide, mirroring the host's finance.Publish/Current singleton.
func Set(l CreditLedger) {
	mu.Lock()
	injected = l
	mu.Unlock()
}

// Get returns the injected ledger, or nil when commerce runs standalone (datastore
// fallback). The credit + balance handlers call it on every request.
func Get() CreditLedger {
	mu.RLock()
	defer mu.RUnlock()
	return injected
}
