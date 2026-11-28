package kraken

import (
	"context"
	"strings"
	"time"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/payment/processor"
)

const (
	walletIDPrefix       = "kraken:"
	defaultPendingLease  = 30 * time.Minute
	defaultIdleEvictTime = 15 * time.Minute
)

var defaultWatchedPairs = []processor.CryptoAssetPair{
	{Chain: "base", Token: "usdc"},
	{Chain: "ethereum", Token: "usdc"},
	{Chain: "arbitrum", Token: "usdc"},
	{Chain: "optimism", Token: "usdc"},
	{Chain: "polygon", Token: "usdc"},
	{Chain: "avalanche", Token: "usdc"},
	{Chain: "solana", Token: "usdc"},
	{Chain: "ethereum", Token: "usdt"},
	{Chain: "arbitrum", Token: "usdt"},
	{Chain: "optimism", Token: "usdt"},
	{Chain: "polygon", Token: "usdt"},
	{Chain: "avalanche", Token: "usdt"},
	{Chain: "solana", Token: "usdt"},
}

// RequiredSecretRefs implements processor.ManagedAccountProcessor.
func (p *Processor) RequiredSecretRefs(secretRef string) []string {
	folder := strings.TrimRight(strings.TrimSpace(secretRef), "/")
	return []string{
		folder + "/KRAKEN_API_KEY",
		folder + "/KRAKEN_API_SECRET",
	}
}

// ConfigureAccount implements processor.ManagedAccountProcessor.
func (p *Processor) ConfigureAccount(ctx context.Context, endpoint, secretRef string, readSecret processor.SecretReader) bool {
	if readSecret == nil || strings.TrimSpace(secretRef) == "" {
		p.ClearAccount()
		return false
	}
	folder := strings.TrimRight(strings.TrimSpace(secretRef), "/")
	apiKey := readSecret(ctx, folder+"/KRAKEN_API_KEY")
	apiSecret := readSecret(ctx, folder+"/KRAKEN_API_SECRET")
	otp := readSecret(ctx, folder+"/KRAKEN_OTP")
	if apiKey == "" || apiSecret == "" {
		p.ClearAccount()
		return false
	}
	ep := strings.TrimSpace(endpoint)
	if ep == "" {
		ep = defaultBaseURL
	}
	p.Configure(Config{
		APIKey:    apiKey,
		APISecret: apiSecret,
		APIURL:    ep,
		OTP:       otp,
	})
	return p.IsAvailable(ctx)
}

// ClearAccount implements processor.ManagedAccountProcessor.
func (p *Processor) ClearAccount() {
	p.Configure(Config{APIURL: defaultBaseURL})
}

// SeedSecretEnvNames implements processor.ManagedAccountProcessor.
func (p *Processor) SeedSecretEnvNames() []string {
	return []string{"KRAKEN_API_KEY"}
}

// SeedAccountFromEnv implements processor.ManagedAccountProcessor.
func (p *Processor) SeedAccountFromEnv(env map[string]string, resolveFolder func(secretEnvName string) string) (processor.SeededAccount, bool, bool) {
	hasCreds := env["KRAKEN_API_KEY"] != "" && env["KRAKEN_API_SECRET"] != ""
	settings := make(map[string]string)
	for k, v := range env {
		rest, ok := strings.CutPrefix(k, "KRAKEN_DEPOSIT_METHOD_")
		if !ok {
			continue
		}
		parts := strings.Split(strings.ToLower(rest), "_")
		settings["depositMethod."+strings.Join(parts, ".")] = v
	}
	if !hasCreds && len(settings) == 0 {
		return processor.SeededAccount{}, false, false
	}
	acct := processor.SeededAccount{
		ID:       "exchange",
		Endpoint: env["KRAKEN_API_URL"],
		On:       true,
		Settings: settings,
	}
	if hasCreds && resolveFolder != nil {
		acct.SecretRef = resolveFolder("KRAKEN_API_KEY")
	}
	return acct, hasCreds, true
}

// OwnsWallet implements processor.DepositPoolProcessor.
func (p *Processor) OwnsWallet(walletID string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(walletID)), walletIDPrefix)
}

// PoolLeaseTerms implements processor.DepositPoolProcessor.
func (p *Processor) PoolLeaseTerms() (time.Duration, time.Duration) {
	return defaultPendingLease, defaultIdleEvictTime
}

// WatchedPairs implements processor.DepositPoolProcessor.
func (p *Processor) WatchedPairs() ([]processor.CryptoAssetPair, string) {
	out := make([]processor.CryptoAssetPair, len(defaultWatchedPairs))
	copy(out, defaultWatchedPairs)
	ep := p.baseURL
	if ep == "" {
		ep = defaultBaseURL
	}
	return out, ep
}

// WarmPools implements processor.DepositPoolProcessor.
func (p *Processor) WarmPools(ctx context.Context, pairs []processor.CryptoAssetPair) error {
	if !p.IsAvailable(ctx) {
		return nil
	}
	seenFamily := make(map[string]bool)
	var firstErr error
	for _, pair := range pairs {
		fam := coarseFamily(pair.Chain, pair.Token)
		if seenFamily[fam] {
			continue
		}
		seenFamily[fam] = true
		n, err := p.EnsurePool(ctx, pair.Chain, pair.Token, DefaultAddressPoolSize)
		if err != nil {
			log.Warn("deposit pool warm (%s %s/%s, %d/%d): %v", p.Type(), pair.Chain, pair.Token, n, DefaultAddressPoolSize, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		log.Info("deposit pool ready (%s %s/%s: %d/%d unused addresses)", p.Type(), pair.Chain, pair.Token, n, DefaultAddressPoolSize)
	}
	return firstErr
}

func coarseFamily(chain, token string) string {
	switch strings.ToLower(strings.TrimSpace(chain)) {
	case "ethereum", "base", "arbitrum", "optimism", "polygon", "avalanche":
		return "evm"
	case "solana":
		return "solana"
	default:
		return strings.ToLower(strings.TrimSpace(chain)) + ":" + strings.ToLower(strings.TrimSpace(token))
	}
}

// ReplenishPools implements processor.DepositPoolProcessor.
func (p *Processor) ReplenishPools(ctx context.Context) error {
	return p.ReplenishPendingPools(ctx)
}

// RetireAddress implements processor.DepositPoolProcessor.
func (p *Processor) RetireAddress(ctx context.Context, chain, token, address string) error {
	return p.RetireAndRotate(ctx, chain, token, address)
}

// ListInboundDeposits implements processor.DepositPoolProcessor.
func (p *Processor) ListInboundDeposits(ctx context.Context, chain, token string) ([]processor.InboundDeposit, error) {
	records, err := p.ListDeposits(ctx, chain, token)
	if err != nil {
		return nil, err
	}
	out := make([]processor.InboundDeposit, 0, len(records))
	for _, r := range records {
		status := strings.ToLower(strings.TrimSpace(r.Status))
		var confirmed bool
		switch status {
		case "success", "settled":
			confirmed = true
		case "pending", "initial":
			confirmed = false
		default:
			continue
		}
		txHash := strings.TrimSpace(r.TxID)
		if txHash == "" {
			txHash = strings.TrimSpace(r.RefID)
		}
		out = append(out, processor.InboundDeposit{
			RefID:     strings.TrimSpace(r.RefID),
			TxHash:    txHash,
			Address:   strings.TrimSpace(r.Info),
			Amount:    strings.TrimSpace(r.Amount),
			Timestamp: r.Time,
			Confirmed: confirmed,
		})
	}
	return out, nil
}

// CachedMinimumCents implements processor.DepositPoolProcessor.
func (p *Processor) CachedMinimumCents(chain, token string) int64 {
	return p.CachedMinimumUSDCents(chain, token)
}

var (
	_ processor.ManagedAccountProcessor = (*Processor)(nil)
	_ processor.DepositPoolProcessor    = (*Processor)(nil)
)
