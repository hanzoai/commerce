package depositledger

import (
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
)

// accountsOf is the deposit accounts a table written as CRYPTO_DEPOSIT_* pairs describes, so a
// test can state a whole deployment in one literal.
func accountsOf(pairs []string) []routing.Account {
	rpc, tok := map[string]string{}, map[string]string{}
	var order []string
	for _, kv := range pairs {
		k, v, _ := strings.Cut(kv, "=")
		switch {
		case strings.HasPrefix(k, "CRYPTO_DEPOSIT_RPC_"):
			rpc[strings.ToLower(strings.TrimPrefix(k, "CRYPTO_DEPOSIT_RPC_"))] = v
		case strings.HasPrefix(k, "CRYPTO_DEPOSIT_TOKEN_"):
			tok[strings.ToLower(strings.TrimPrefix(k, "CRYPTO_DEPOSIT_TOKEN_"))] = v
			order = append(order, strings.ToLower(strings.TrimPrefix(k, "CRYPTO_DEPOSIT_TOKEN_")))
		}
	}
	var out []routing.Account
	for _, key := range order {
		i := strings.LastIndexByte(key, '_')
		net, asset := key[:i], key[i+1:]
		out = append(out, routing.Account{Processor: string(processor.MPC), ID: key, Network: net, Asset: asset, Token: tok[key], Endpoint: rpc[net], On: true})
	}
	return out
}
