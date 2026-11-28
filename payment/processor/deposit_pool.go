package processor

import (
	"context"
	"time"
)

// CryptoAssetPair names a (chain, token) asset pair accepted by a deposit processor.
type CryptoAssetPair struct {
	Chain string
	Token string
}

// InboundDeposit is a normalized inbound crypto deposit reported by a custody or
// exchange processor's status API (independent of direct chain RPC log scanning).
type InboundDeposit struct {
	RefID     string
	TxHash    string
	Address   string
	Tag       string
	Amount    string // decimal amount in token/USD (e.g. "25.00")
	Timestamp int64
	Confirmed bool // true = settled by provider, false = sighted/pending
}

// SecretReader resolves a KMS secret path to its plaintext value at runtime.
type SecretReader func(ctx context.Context, path string) string

// SeededAccount describes a policy account synthesized from legacy environment
// variables during first-boot seeding.
type SeededAccount struct {
	ID        string
	Endpoint  string
	SecretRef string
	On        bool
	Settings  map[string]string
}

// ManagedAccountProcessor is implemented by custody or exchange processors whose
// credentials live under a policy account's SecretRef. Keeping secret key names,
// env-seed translation, and credential parsing inside the provider package keeps
// core billing and depositledger completely free of provider-specific details.
type ManagedAccountProcessor interface {
	PaymentProcessor

	// RequiredSecretRefs returns the concrete KMS paths that must resolve when an
	// admin validates a policy account with secretRef for this processor.
	RequiredSecretRefs(secretRef string) []string

	// ConfigureAccount loads runtime credentials from secretRef + endpoint via
	// readSecret and reports whether the processor is now configured and available.
	ConfigureAccount(ctx context.Context, endpoint, secretRef string, readSecret SecretReader) bool

	// ClearAccount clears runtime credentials when no policy account for this
	// processor is enabled.
	ClearAccount()

	// SeedSecretEnvNames returns legacy environment variable names that anchor KMS
	// folder discovery during first-boot seeding.
	SeedSecretEnvNames() []string

	// SeedAccountFromEnv inspects legacy environment variables on first boot and
	// returns a seeded account when present.
	SeedAccountFromEnv(env map[string]string, resolveFolder func(secretEnvName string) string) (acct SeededAccount, enableProcessor bool, ok bool)
}

// DepositPoolProcessor is implemented by CryptoProcessors that manage a rolling
// pool of rotatable deposit addresses, report inbound deposits via provider APIs,
// and/or surface provider-enforced deposit minimums.
type DepositPoolProcessor interface {
	CryptoProcessor

	// OwnsWallet reports whether walletID was minted by this processor.
	OwnsWallet(walletID string) bool

	// PoolLeaseTerms returns how long an un-sighted Pending intent holds an address
	// from this processor's pool (0 uses the default standing TTL) and after how
	// long an idle un-sighted intent may be evicted when all pool slots are held.
	PoolLeaseTerms() (pendingTTL, idleEvictAfter time.Duration)

	// WatchedPairs returns the default (chain, token) pairs this processor accepts
	// deposits on and its API origin for diagnostics.
	WatchedPairs() (pairs []CryptoAssetPair, endpoint string)

	// WarmPools pre-warms the rolling address pools for the given watched pairs.
	WarmPools(ctx context.Context, pairs []CryptoAssetPair) error

	// ReplenishPools tops up any rolling address pools awaiting provider sweep completion.
	ReplenishPools(ctx context.Context) error

	// RetireAddress permanently retires a settled address from the active pool and
	// rotates a fresh replacement address into the pool.
	RetireAddress(ctx context.Context, chain, token, address string) error

	// ListInboundDeposits queries the provider's status API for recent inbound deposits.
	ListInboundDeposits(ctx context.Context, chain, token string) ([]InboundDeposit, error)

	// CachedMinimumCents returns the provider's minimum deposit in USD cents for
	// (chain, token) without blocking, or 0 if none applies.
	CachedMinimumCents(chain, token string) int64
}

// ManagedAccountProcessors returns all registered processors implementing ManagedAccountProcessor.
func ManagedAccountProcessors() []ManagedAccountProcessor {
	regs := Global().RegisteredProcessors()
	out := make([]ManagedAccountProcessor, 0, len(regs))
	for _, p := range regs {
		if mp, ok := p.(ManagedAccountProcessor); ok {
			out = append(out, mp)
		}
	}
	return out
}

// DepositPoolProcessors returns all registered processors implementing DepositPoolProcessor.
func DepositPoolProcessors() []DepositPoolProcessor {
	regs := Global().RegisteredProcessors()
	out := make([]DepositPoolProcessor, 0, len(regs))
	for _, p := range regs {
		if dp, ok := p.(DepositPoolProcessor); ok {
			out = append(out, dp)
		}
	}
	return out
}

// PoolOwnerOfWallet returns the registered DepositPoolProcessor that minted walletID, or nil.
func PoolOwnerOfWallet(walletID string) DepositPoolProcessor {
	if walletID == "" {
		return nil
	}
	for _, dp := range DepositPoolProcessors() {
		if dp.OwnsWallet(walletID) {
			return dp
		}
	}
	return nil
}

// IsDepositPoolProcessor reports whether name refers to a registered DepositPoolProcessor.
func IsDepositPoolProcessor(name string) bool {
	if name == "" {
		return false
	}
	p, err := Global().Registered(ProcessorType(name))
	if err != nil {
		return false
	}
	_, ok := p.(DepositPoolProcessor)
	return ok
}

// MerchantAccountLocation describes one merchant sub-account/location reported by a
// card processor (e.g. an active terminal/storefront location) without exposing any
// provider-specific struct to core billing views.
type MerchantAccountLocation struct {
	ID       string
	Country  string
	Currency string
	Cards    bool
	Active   bool
}

// MerchantLocationLister is implemented by card processors that expose live merchant
// locations to the admin routing view and per-currency location diagnostics.
type MerchantLocationLister interface {
	PaymentProcessor
	ActiveLocation(ctx context.Context) (MerchantAccountLocation, bool)
	MerchantLocations(ctx context.Context) ([]MerchantAccountLocation, error)
}
