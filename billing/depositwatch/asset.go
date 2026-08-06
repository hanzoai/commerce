// Package depositwatch is the other half of the crypto top-up rail: the part
// that watches the addresses api/billing/crypto_deposit.go hands out and turns
// money that actually arrived into spendable balance.
//
// Without it the rail is a hole — an address is minted, a customer sends real
// USDC to it, and nothing in the process ever looks at the chain, so the intent
// sits Pending forever and the money is simply gone. That is why the rail is
// switched off (cryptoDepositsCanBeCredited), and this package is what earns the
// right to switch it on.
//
// It is DECOMPLECTED the same way billing/husdindex is, and for the same reason:
//
//   - this package is pure policy over small interfaces — which blocks to scan,
//     how deep is deep enough, what a token amount is worth, which address
//     belongs to whom. No datastore, no HTTP handler, no cgo, so every money
//     decision in it is unit-tested against fakes with no chain and no database.
//   - billing/depositledger is the I/O half: the per-org intent store, the
//     idempotent ledger write, the persisted cursor, and the schedule.
//
// The chain READS come from billing/husdindex.Client, which is this repo's one
// ERC-20 JSON-RPC read client. Its package is named for its first caller, not
// for a restriction — it is parameterized by (rpcURL, tokenAddr) and is just as
// correct pointed at USDC on Base as at HUSD on the Hanzo EVM. Reaching for it
// instead of writing a second JSON-RPC client is the whole point.
//
// SCOPE: EVM chains, ERC-20, dollar-pegged tokens. Everything outside that is
// refused rather than approximated — see pegCents and creditableTokens below.
package depositwatch

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// Asset is ONE watched (chain, token) pair: where its ERC-20 contract lives and
// what one whole token is worth. Decimals are deliberately NOT here — they are
// read from the contract at scan time (Watcher.verify), because a decimals
// constant is a number no code can check and a wrong one credits 10^12 times
// too much.
type Asset struct {
	Chain    string // "ethereum", "base", … (lowercase; matches the intent's Chain)
	Token    string // "usdc", "usdt" (lowercase; matches the intent's Token)
	Contract string // ERC-20 contract address, lowercased 0x
	RPCURL   string // JSON-RPC endpoint for Chain
}

// Key identifies an asset for cursors and maps.
func (a Asset) Key() string { return a.Chain + ":" + a.Token }

// PegCents is what one whole token of this asset is worth in USD cents.
func (a Asset) PegCents() int64 { return pegCents[a.Token] }

// pegCents is THE list of tokens this rail will credit, and what one whole token
// is worth.
//
// A deposit is credited at a FIXED dollar peg and never at a market rate,
// because commerce has no price oracle and inventing one to value a customer's
// money would be guessing. That is not a gap to fill later with a price feed
// bolted onto this file; it is the reason the creditable set is stablecoins:
//
//	ETH / MATIC / AVAX / BNB — a native coin cannot even be OBSERVED here (a
//	  native transfer emits no log, so eth_getLogs cannot see it) and cannot be
//	  priced. Two independent blockers, so it is not watched at all.
//	Anything else pegged — add it here WITH its peg, deliberately.
//
// The known, bounded, deliberate risk this accepts: a depegged stablecoin is
// credited above its market value. That is the standard bargain every payment
// rail makes for stablecoins, and it is stated here rather than hidden in a
// rate table nobody reads.
var pegCents = map[string]int64{
	"usdc": 100,
	"usdt": 100,
}

// PegRate renders an asset's peg the way the intent records an exchange rate
// ("1.00" for a dollar-pegged token).
func (a Asset) PegRate() string {
	p := a.PegCents()
	return fmt.Sprintf("%d.%02d", p/100, p%100)
}

// envRPCPrefix and envTokenPrefix are the environment (KMS-injected) keys the
// watch table is read from:
//
//	CRYPTO_DEPOSIT_RPC_<CHAIN>            JSON-RPC endpoint for that chain
//	CRYPTO_DEPOSIT_TOKEN_<CHAIN>_<TOKEN>  ERC-20 contract for that token there
//
// e.g. CRYPTO_DEPOSIT_RPC_BASE, CRYPTO_DEPOSIT_TOKEN_BASE_USDC.
//
// This mirrors util/husd exactly, including the part that matters: a contract
// address has NO DEFAULT. An unset deploy watches nothing instead of guessing at
// an address, and a token contract is per-chain anyway (USDC on BSC is a
// different contract with different decimals than USDC on Ethereum), so a
// built-in table would be a list of constants nobody can verify from here.
const (
	envRPCPrefix   = "CRYPTO_DEPOSIT_RPC_"
	envTokenPrefix = "CRYPTO_DEPOSIT_TOKEN_"
)

// AssetsFromEnv builds the watch table from environ (as returned by os.Environ:
// "KEY=VALUE" strings). Passing environ in rather than reading os keeps this
// package pure and lets the tests state a whole deployment in one literal.
//
// It fails CLOSED on an incoherent table — a token configured on a chain with no
// RPC endpoint, or a token this rail cannot price — because a half-configured
// money rail that silently watches less than the operator asked for is exactly
// the failure this whole package exists to end.
func AssetsFromEnv(environ []string) ([]Asset, error) {
	rpc := map[string]string{}
	type tok struct{ chain, token, contract string }
	var toks []tok

	for _, kv := range environ {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		key, val := kv[:eq], strings.TrimSpace(kv[eq+1:])
		if val == "" {
			continue
		}
		switch {
		case strings.HasPrefix(key, envTokenPrefix):
			// <CHAIN>_<TOKEN>: the token is the last segment, so a chain name is
			// free to contain an underscore even though none of ours does.
			rest := key[len(envTokenPrefix):]
			us := strings.LastIndexByte(rest, '_')
			if us <= 0 || us == len(rest)-1 {
				return nil, fmt.Errorf("depositwatch: %s is not %s<CHAIN>_<TOKEN>", key, envTokenPrefix)
			}
			toks = append(toks, tok{
				chain:    strings.ToLower(rest[:us]),
				token:    strings.ToLower(rest[us+1:]),
				contract: strings.ToLower(val),
			})
		case strings.HasPrefix(key, envRPCPrefix):
			rpc[strings.ToLower(key[len(envRPCPrefix):])] = val
		}
	}

	assets := make([]Asset, 0, len(toks))
	seen := map[string]bool{}
	for _, t := range toks {
		if _, ok := pegCents[t.token]; !ok {
			return nil, fmt.Errorf("depositwatch: %s%s_%s configures token %q, which has no USD peg — this rail credits only dollar-pegged tokens (%s); remove it or add its peg deliberately",
				envTokenPrefix, strings.ToUpper(t.chain), strings.ToUpper(t.token), t.token, strings.Join(creditableTokens(), ", "))
		}
		if !isHexAddress(t.contract) {
			return nil, fmt.Errorf("depositwatch: %s%s_%s is not a 20-byte hex address: %q",
				envTokenPrefix, strings.ToUpper(t.chain), strings.ToUpper(t.token), t.contract)
		}
		url, ok := rpc[t.chain]
		if !ok {
			return nil, fmt.Errorf("depositwatch: token %s is configured on chain %q but %s%s is unset — a watched token with no endpoint is money nobody is watching",
				t.token, t.chain, envRPCPrefix, strings.ToUpper(t.chain))
		}
		a := Asset{Chain: t.chain, Token: t.token, Contract: t.contract, RPCURL: url}
		if seen[a.Key()] {
			continue
		}
		seen[a.Key()] = true
		assets = append(assets, a)
	}
	// Deterministic order so a boot log, a status read and a test all agree.
	sort.Slice(assets, func(i, j int) bool { return assets[i].Key() < assets[j].Key() })
	return assets, nil
}

// creditableTokens lists the tokens with a declared peg, sorted.
func creditableTokens() []string {
	out := make([]string, 0, len(pegCents))
	for t := range pegCents {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// isHexAddress reports whether s is a 0x-prefixed 20-byte hex address.
func isHexAddress(s string) bool {
	if !strings.HasPrefix(s, "0x") || len(s) != 42 {
		return false
	}
	for _, r := range s[2:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}

// ErrDust is returned by AmountCents for a transfer worth less than one cent.
// It is not a failure: sub-cent dust is a real transfer that credits nothing.
var ErrDust = errors.New("depositwatch: transfer is worth less than one cent")

// AmountCents converts a raw ERC-20 amount into USD cents at the asset's peg:
//
//	cents = units × pegCents / 10^decimals
//
// Truncating, always DOWN. A sub-cent remainder is dropped rather than rounded
// up, so the rail can never credit value that was not sent; the dust stays on
// chain in the custody address where it can still be swept.
//
// decimals comes from the token contract (Client.Decimals), never from config.
// This is the arithmetic that turns a 6-decimal token read as 18 into a credit
// 10^12 too large, so it refuses every input it cannot justify rather than
// producing a number: a nil or negative amount, an implausible decimals, an
// unpegged token, and a result that does not fit in the ledger's int64 cents.
func AmountCents(units *big.Int, decimals int, pegCents int64) (int64, error) {
	if units == nil || units.Sign() < 0 {
		return 0, errors.New("depositwatch: transfer amount must be non-negative")
	}
	// A token with fewer than 2 decimals cannot express a cent, and one with
	// more than 36 is outside anything the ERC-20 world has produced — either is
	// a contract we do not understand well enough to credit.
	if decimals < 2 || decimals > 36 {
		return 0, fmt.Errorf("depositwatch: unusable token decimals %d", decimals)
	}
	if pegCents <= 0 {
		return 0, fmt.Errorf("depositwatch: token has no USD peg")
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	cents := new(big.Int).Quo(new(big.Int).Mul(units, big.NewInt(pegCents)), scale)
	if !cents.IsInt64() {
		return 0, fmt.Errorf("depositwatch: %s base units is %s cents, which overflows the ledger", units, cents)
	}
	if cents.Sign() == 0 {
		return 0, ErrDust
	}
	return cents.Int64(), nil
}
