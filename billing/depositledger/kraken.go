package depositledger

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/kraken"
	"github.com/hanzoai/commerce/payment/routing"
	"github.com/hanzoai/commerce/secrets"
)

var defaultKrakenPairs = []struct {
	chain string
	token string
}{
	{chain: "base", token: "usdc"},
	{chain: "ethereum", token: "usdc"},
	{chain: "arbitrum", token: "usdc"},
	{chain: "optimism", token: "usdc"},
	{chain: "polygon", token: "usdc"},
	{chain: "avalanche", token: "usdc"},
	{chain: "solana", token: "usdc"},
	{chain: "ethereum", token: "usdt"},
	{chain: "arbitrum", token: "usdt"},
	{chain: "optimism", token: "usdt"},
	{chain: "polygon", token: "usdt"},
	{chain: "avalanche", token: "usdt"},
	{chain: "solana", token: "usdt"},
}

// krakenFromAccounts is the exchange processor and the assets it takes deposits in, when
// the policy has a kraken account on. It configures and returns the global registered
// *kraken.Processor so HTTP checkouts and depositledger share one monotonic API nonce,
// one rolling 5-address pool cursor, one retired-address set and one method cache.
func krakenFromAccounts(ctx context.Context, accounts []routing.Account) (*kraken.Processor, []depositwatch.Asset) {
	for _, a := range accounts {
		if !a.On || a.Processor != string(processor.Kraken) || a.SecretRef == "" {
			continue
		}
		folder := strings.TrimRight(a.SecretRef, "/")
		cred := struct{ APIKey, APISecret, OTP string }{
			APIKey:    secrets.String(ctx, folder+"/KRAKEN_API_KEY"),
			APISecret: secrets.String(ctx, folder+"/KRAKEN_API_SECRET"),
			OTP:       secrets.String(ctx, folder+"/KRAKEN_OTP"),
		}
		if cred.APIKey == "" || cred.APISecret == "" {
			continue
		}
		cfg := kraken.Config{APIKey: cred.APIKey, APISecret: cred.APISecret, APIURL: a.Endpoint, OTP: cred.OTP}
		var kp *kraken.Processor
		if reg, err := processor.Global().Registered(processor.Kraken); err == nil {
			if shared, ok := reg.(*kraken.Processor); ok {
				shared.Configure(cfg)
				kp = shared
			}
		}
		if kp == nil {
			kp = kraken.NewProcessor(cfg)
		}
		if !kp.IsAvailable(ctx) {
			continue
		}
		endpoint := "https://api.kraken.com"
		if a.Endpoint != "" {
			endpoint = a.Endpoint
		}
		assets := make([]depositwatch.Asset, 0, len(defaultKrakenPairs))
		for _, p := range defaultKrakenPairs {
			assets = append(assets, depositwatch.Asset{Chain: p.chain, Token: p.token, Contract: "kraken:" + p.chain + ":" + p.token, RPCURL: endpoint})
		}
		return kp, assets
	}
	return nil, nil
}

func warmKrakenAddressPools(ctx context.Context, st *state) {
	if st == nil || st.kraken == nil || !st.poolsWarmed.CompareAndSwap(false, true) {
		return
	}
	seenFamily := make(map[string]bool)
	for _, a := range st.assets {
		if !strings.HasPrefix(a.Contract, "kraken:") {
			continue
		}
		fam := "evm"
		if strings.EqualFold(a.Chain, "solana") {
			fam = "solana"
		} else if !isEVMChainName(a.Chain) {
			fam = strings.ToLower(a.Chain) + ":" + strings.ToLower(a.Token)
		}
		if seenFamily[fam] {
			continue
		}
		seenFamily[fam] = true
		if n, err := st.kraken.EnsurePool(ctx, a.Chain, a.Token, kraken.DefaultAddressPoolSize); err != nil {
			log.Warn("depositledger: kraken pool warm for %s/%s (%d/%d): %v", a.Chain, a.Token, n, kraken.DefaultAddressPoolSize, err)
		} else {
			log.Info("depositledger: kraken %s/%s deposit pool ready (%d/%d unused addresses)", a.Chain, a.Token, n, kraken.DefaultAddressPoolSize)
		}
	}
}

func isEVMChainName(chain string) bool {
	switch strings.ToLower(strings.TrimSpace(chain)) {
	case "ethereum", "base", "arbitrum", "optimism", "polygon", "avalanche":
		return true
	default:
		return false
	}
}

func (s *Service) syncKraken(ctx context.Context, st *state) (int, error) {
	if s == nil || st.kraken == nil || s.store == nil {
		return 0, nil
	}

	warmKrakenAddressPools(ctx, st)
	_ = st.kraken.ReplenishPendingPools(ctx)

	type watchedPair struct {
		chain string
		token string
		items []depositwatch.Watched
	}
	byToken := make(map[string][]watchedPair)
	seenPair := make(map[string]bool)

	for _, a := range st.assets {
		key := a.Chain + ":" + a.Token
		if seenPair[key] {
			continue
		}
		seenPair[key] = true
		watched, err := s.store.Watched(ctx, a.Chain, a.Token)
		if err != nil {
			return 0, err
		}
		var krakenWatched []depositwatch.Watched
		for _, w := range watched {
			if strings.HasPrefix(w.Wallet, "kraken:") || strings.HasPrefix(a.Contract, "kraken:") {
				krakenWatched = append(krakenWatched, w)
			}
		}
		if len(krakenWatched) > 0 {
			byToken[a.Token] = append(byToken[a.Token], watchedPair{
				chain: a.Chain,
				token: a.Token,
				items: krakenWatched,
			})
		}
	}

	creditedCount := 0
	for token, pairs := range byToken {
		records, err := st.kraken.ListDeposits(ctx, "", token)
		if err != nil {
			return creditedCount, err
		}
		if len(records) == 0 {
			continue
		}
		for _, pair := range pairs {
			reqConf := cryptopaymentintent.RequiredConfirmationsForChain(cryptopaymentintent.Chain(pair.chain))
			for _, w := range pair.items {
				for _, r := range records {
					if !strings.EqualFold(strings.TrimSpace(r.Info), strings.TrimSpace(w.Address)) {
						continue
					}
					if r.Time > 0 && !w.CreatedAt.IsZero() && time.Unix(r.Time, 0).Before(w.CreatedAt.Add(-10*time.Minute)) {
						continue
					}
					status := strings.ToLower(strings.TrimSpace(r.Status))
					txHash := strings.TrimSpace(r.TxID)
					if txHash == "" {
						txHash = strings.TrimSpace(r.RefID)
					}
					if w.Status == cryptopaymentintent.Succeeded && sameTxHash(w.TxHash, txHash) {
						continue
					}
					switch status {
					case "pending", "initial":
						_ = s.store.Sight(ctx, depositwatch.Sighting{
							Org:           w.Org,
							IntentID:      w.IntentID,
							TxHash:        txHash,
							Block:         uint64(r.Time),
							Confirmations: 1,
						})
					case "success", "settled":
						cents, err := currency.USD.Parse(strings.TrimSpace(r.Amount))
						if err != nil || cents <= 0 {
							continue
						}
						dedup := fmt.Sprintf("kraken:%s:%s:%s", pair.chain, r.RefID, txHash)
						wrote, err := s.store.Credit(ctx, depositwatch.Credit{
							DedupKey:      dedup,
							Org:           w.Org,
							Test:          w.Test,
							IntentID:      w.IntentID,
							Subject:       w.Subject,
							Chain:         pair.chain,
							Token:         pair.token,
							TxHash:        txHash,
							Block:         uint64(r.Time),
							Confirmations: reqConf,
							Units:         strings.TrimSpace(r.Amount),
							AmountCents:   int64(cents),
							PegRate:       "1.00",
						})
						if err != nil {
							return creditedCount, err
						}
						if wrote {
							creditedCount++
						}
					}
				}
			}
		}
	}
	return creditedCount, nil
}
