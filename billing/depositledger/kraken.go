package depositledger

import (
	"context"
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/providers/kraken"
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

func krakenFromEnv(environ []string) (*kraken.Processor, []depositwatch.Asset) {
	var apiKey, apiSecret, apiURL, otp string
	for _, entry := range environ {
		k, v, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch k {
		case "KRAKEN_API_KEY":
			apiKey = v
		case "KRAKEN_API_SECRET":
			apiSecret = v
		case "KRAKEN_API_URL":
			apiURL = v
		case "KRAKEN_OTP":
			otp = v
		}
	}
	if apiKey == "" || apiSecret == "" {
		return nil, nil
	}
	kp := kraken.NewProcessor(kraken.Config{
		APIKey:    apiKey,
		APISecret: apiSecret,
		APIURL:    apiURL,
		OTP:       otp,
	})
	if !kp.IsAvailable(context.Background()) {
		return nil, nil
	}
	endpoint := "https://api.kraken.com"
	if apiURL != "" {
		endpoint = apiURL
	}
	assets := make([]depositwatch.Asset, 0, len(defaultKrakenPairs))
	for _, p := range defaultKrakenPairs {
		assets = append(assets, depositwatch.Asset{
			Chain:    p.chain,
			Token:    p.token,
			Contract: "kraken:" + p.chain + ":" + p.token,
			RPCURL:   endpoint,
		})
	}
	return kp, assets
}

func (s *Service) syncKraken(ctx context.Context) (int, error) {
	if s == nil || s.kraken == nil || s.store == nil {
		return 0, nil
	}

	type watchedPair struct {
		chain string
		token string
		items []depositwatch.Watched
	}
	byToken := make(map[string][]watchedPair)
	seenPair := make(map[string]bool)

	for _, a := range s.assets {
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
		records, err := s.kraken.ListDeposits(ctx, "", token)
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
					status := strings.ToLower(strings.TrimSpace(r.Status))
					txHash := strings.TrimSpace(r.TxID)
					if txHash == "" {
						txHash = strings.TrimSpace(r.RefID)
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
