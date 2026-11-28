package depositledger

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	_ "github.com/hanzoai/commerce/payment/providers"
	"github.com/hanzoai/commerce/payment/routing"
	"github.com/hanzoai/commerce/secrets"
)

const poolContractPrefix = "pool:"

// poolProcessorsFromAccounts configures any registered DepositPoolProcessor whose
// policy account is enabled and returns the active pool processors alongside the
// (chain, token) assets they accept deposits on. Core depositledger knows only the
// processor interfaces, never any concrete exchange or custody provider package.
func poolProcessorsFromAccounts(ctx context.Context, accounts []routing.Account) ([]processor.DepositPoolProcessor, []depositwatch.Asset) {
	var pools []processor.DepositPoolProcessor
	var assets []depositwatch.Asset

	for _, a := range accounts {
		if !a.On || strings.TrimSpace(a.SecretRef) == "" {
			continue
		}
		reg, err := processor.Global().Registered(processor.ProcessorType(a.Processor))
		if err != nil {
			continue
		}
		mp, okM := reg.(processor.ManagedAccountProcessor)
		dp, okD := reg.(processor.DepositPoolProcessor)
		if !okM || !okD {
			continue
		}
		if !mp.ConfigureAccount(ctx, a.Endpoint, a.SecretRef, secrets.String) {
			continue
		}
		pairs, ep := dp.WatchedPairs()
		if strings.TrimSpace(a.Endpoint) != "" {
			ep = strings.TrimSpace(a.Endpoint)
		}
		for _, p := range pairs {
			assets = append(assets, depositwatch.Asset{
				Chain:    p.Chain,
				Token:    p.Token,
				Contract: fmt.Sprintf("%s%s:%s:%s", poolContractPrefix, dp.Type(), p.Chain, p.Token),
				RPCURL:   ep,
			})
		}
		pools = append(pools, dp)
	}
	return pools, assets
}

func warmDepositPools(ctx context.Context, st *state) {
	if st == nil || len(st.pools) == 0 || !st.poolsWarmed.CompareAndSwap(false, true) {
		return
	}
	for _, dp := range st.pools {
		prefix := fmt.Sprintf("%s%s:", poolContractPrefix, dp.Type())
		var pairs []processor.CryptoAssetPair
		for _, a := range st.assets {
			if strings.HasPrefix(a.Contract, prefix) {
				pairs = append(pairs, processor.CryptoAssetPair{Chain: a.Chain, Token: a.Token})
			}
		}
		if len(pairs) == 0 {
			pairs, _ = dp.WatchedPairs()
		}
		_ = dp.WarmPools(ctx, pairs)
	}
}

func (s *Service) syncPools(ctx context.Context, st *state) (int, error) {
	if s == nil || st == nil || len(st.pools) == 0 || s.store == nil {
		return 0, nil
	}

	warmDepositPools(ctx, st)

	totalCredited := 0
	for _, dp := range st.pools {
		_ = dp.ReplenishPools(ctx)
		n, err := s.syncPoolProcessor(ctx, st, dp)
		totalCredited += n
		if err != nil {
			return totalCredited, err
		}
	}
	return totalCredited, nil
}

func (s *Service) syncPoolProcessor(ctx context.Context, st *state, dp processor.DepositPoolProcessor) (int, error) {
	type watchedPair struct {
		chain string
		token string
		items []depositwatch.Watched
	}
	byToken := make(map[string][]watchedPair)
	seenPair := make(map[string]bool)
	contractPrefix := fmt.Sprintf("%s%s:", poolContractPrefix, dp.Type())

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
		var owned []depositwatch.Watched
		for _, w := range watched {
			if dp.OwnsWallet(w.Wallet) || strings.HasPrefix(a.Contract, contractPrefix) {
				owned = append(owned, w)
			}
		}
		if len(owned) > 0 {
			byToken[a.Token] = append(byToken[a.Token], watchedPair{
				chain: a.Chain,
				token: a.Token,
				items: owned,
			})
		}
	}

	creditedCount := 0
	for token, pairs := range byToken {
		deposits, err := dp.ListInboundDeposits(ctx, "", token)
		if err != nil {
			return creditedCount, err
		}
		if len(deposits) == 0 {
			continue
		}
		for _, pair := range pairs {
			reqConf := cryptopaymentintent.RequiredConfirmationsForChain(cryptopaymentintent.Chain(pair.chain))
			for _, w := range pair.items {
				for _, d := range deposits {
					if !strings.EqualFold(strings.TrimSpace(d.Address), strings.TrimSpace(w.Address)) {
						continue
					}
					if d.Timestamp > 0 && !w.CreatedAt.IsZero() && time.Unix(d.Timestamp, 0).Before(w.CreatedAt.Add(-10*time.Minute)) {
						continue
					}
					if w.Status == cryptopaymentintent.Succeeded && sameTxHash(w.TxHash, d.TxHash) {
						continue
					}
					if !d.Confirmed {
						_ = s.store.Sight(ctx, depositwatch.Sighting{
							Org:           w.Org,
							IntentID:      w.IntentID,
							TxHash:        d.TxHash,
							Block:         uint64(d.Timestamp),
							Confirmations: 1,
						})
						continue
					}
					cents, err := currency.USD.Parse(strings.TrimSpace(d.Amount))
					if err != nil || cents <= 0 {
						continue
					}
					dedup := fmt.Sprintf("%s:%s:%s:%s", dp.Type(), pair.chain, d.RefID, d.TxHash)
					wrote, err := s.store.Credit(ctx, depositwatch.Credit{
						DedupKey:      dedup,
						Provider:      string(dp.Type()),
						Org:           w.Org,
						Test:          w.Test,
						IntentID:      w.IntentID,
						Subject:       w.Subject,
						Chain:         pair.chain,
						Token:         pair.token,
						TxHash:        d.TxHash,
						Block:         uint64(d.Timestamp),
						Confirmations: reqConf,
						Units:         strings.TrimSpace(d.Amount),
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
	return creditedCount, nil
}
