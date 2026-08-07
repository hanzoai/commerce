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
// The chain READS come from one read client per chain family:
// billing/husdindex.Client for the EVM (this repo's one ERC-20 JSON-RPC read
// client, named for its first caller and not for a restriction),
// billing/solanarpc.Client for Solana, billing/tonrpc.Client for TON and
// billing/xrplrpc.Client for XRPL. All satisfy Reader, so adding a chain adds a
// reader and touches none of the policy here.
//
// SCOPE: dollar-pegged tokens on chains this package can read. Everything
// outside that is refused rather than approximated — see pegCents,
// creditableTokens and chainFamily below.
package depositwatch

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/hanzoai/commerce/models/cryptopaymentintent"
)

// Asset is ONE watched (chain, token) pair: where the token is defined on chain
// and what one whole token is worth. Decimals are deliberately NOT here — they
// are read from the chain at scan time (Watcher.verify), because a decimals
// constant is a number no code can check and a wrong one credits 10^12 times
// too much.
type Asset struct {
	Chain string // "ethereum", "base", "solana", "ton", "xrpl" (lowercase; matches the intent's Chain)
	Token string // "usdc", "usdt", "rlusd" (lowercase; matches the intent's Token)
	// Contract is what DEFINES the token on its chain, written the way that
	// chain writes it:
	//
	//	EVM     the ERC-20 contract address, lowercased hex
	//	Solana  the SPL mint account, base58 exactly as given
	//	TON     the jetton master address, base64url exactly as given
	//	XRPL    <CURRENCY>.<ISSUER> — because on XRPL neither half is a token on
	//	        its own: an issuer issues many currencies, and a currency code is
	//	        a name anybody may issue
	//
	// Normalising it any other way would break the comparison it exists for.
	Contract string
	RPCURL   string // JSON-RPC endpoint for Chain

	// PooledAddress is the ONE custody account this chain's deposits all land
	// in, for a chain where deposits are POOLED (see Pooled). Empty everywhere
	// else, and it must stay empty there: on a per-payer chain the address is
	// minted per deposit by the custody signer, and a configured one would hand
	// every payer the same address.
	//
	// It is CONFIGURED (CRYPTO_DEPOSIT_ADDRESS_<CHAIN>) rather than minted, and
	// that is the point rather than a shortcut. Asking the custody signer for an
	// XRPL address per deposit is precisely the thing the pooled model exists to
	// avoid — every one of those accounts would strand a non-refundable base
	// reserve — and this repo's signer would not even answer the question: it
	// derives no XRPL key and its GenerateAddress falls through to the EVM
	// default, so a "minted" XRPL address would be an 0x string nobody can pay.
	//
	// It is per-CHAIN and not per-token, because the account is: one r-address
	// holds every issued currency sent to it. Two assets on a pooled chain
	// therefore carry the same string here, read from the same variable.
	//
	// AssetsFromEnv refuses a pooled chain that has no address, so an asset that
	// EXISTS is an asset that can be paid to. That is what lets the mint path
	// treat a missing address as impossible rather than as a case to handle.
	PooledAddress string
}

// Key identifies an asset for cursors and maps.
func (a Asset) Key() string { return a.Chain + ":" + a.Token }

// Family is HOW a chain is read, how it writes an address, and whether one
// address belongs to one payer. It is the one place a per-chain difference is
// allowed to live in this package; everything downstream — the confirmation
// rule, the amount arithmetic, the dedup key — is written once and applies to
// every family.
//
// It is ONE enum rather than a set of IsX() predicates so that every question a
// caller asks about a chain is answered from the same table, and so that adding
// a family makes the switch that builds readers fail to compile rather than
// fall to an EVM default.
type Family int

const (
	FamilyEVM Family = iota
	FamilySolana
	FamilyTON
	FamilyXRPL
)

// chainFamily is the set of chains this rail can READ, keyed by the intent
// model's own chain constants so the two lists cannot drift apart.
//
// A chain that is not here is REFUSED at configuration time rather than handed
// an EVM reader on the assumption that everything is an EVM. That assumption is
// how a Solana endpoint gets asked for eth_getLogs, answers an error every 30
// seconds, and watches nothing while looking configured.
var chainFamily = map[cryptopaymentintent.Chain]Family{
	cryptopaymentintent.Ethereum:  FamilyEVM,
	cryptopaymentintent.Base:      FamilyEVM,
	cryptopaymentintent.Polygon:   FamilyEVM,
	cryptopaymentintent.Arbitrum:  FamilyEVM,
	cryptopaymentintent.Optimism:  FamilyEVM,
	cryptopaymentintent.Avalanche: FamilyEVM,
	cryptopaymentintent.BSC:       FamilyEVM,
	cryptopaymentintent.Lux:       FamilyEVM,
	cryptopaymentintent.Zoo:       FamilyEVM,
	cryptopaymentintent.Solana:    FamilySolana,
	cryptopaymentintent.TON:       FamilyTON,
	cryptopaymentintent.XRPL:      FamilyXRPL,
}

// Family reports how this asset's chain is read and written. It is the one
// question the I/O half must ask to build the right reader, and asking it here
// keeps the chain→family table in a single place.
func (a Asset) Family() Family { return chainFamily[cryptopaymentintent.Chain(a.Chain)] }

// Pooled reports whether this chain shares ONE deposit address across payers,
// so that the address alone cannot say whose money arrived.
//
// It is true for XRPL and false everywhere else, and the reason is economic
// rather than technical: XRPL charges a NON-REFUNDABLE base reserve in XRP for
// every funded account, so minting a fresh address per payer would strand that
// reserve on every deposit, forever. The model the whole ledger uses instead is
// one custody account plus a per-deposit DESTINATION TAG — which is why the
// thing that identifies an intent here is Identity and not Address.
//
// Everywhere else an address is per-payer and free to mint (an EVM address
// costs nothing to derive, a Solana ATA is created by the first deposit itself,
// a TON jetton wallet likewise), so there is nothing to pool and no tag.
func (a Asset) Pooled() bool { return Pooled(a.Chain) }

// Pooled reports whether CHAIN shares one deposit address across payers.
//
// It takes a chain rather than an asset because the MINT side must ask before
// it has one: the question "is this chain's address configured or minted?"
// decides which door a deposit request goes through, and at that moment there
// is a chain and a token and no Asset yet. The pooling fact is per-chain
// anyway — the account is shared by every currency sent to it — so this is the
// primitive and Asset.Pooled is the convenience.
//
// Keeping both on one line of one function is deliberate: the read side and the
// write side must never be able to disagree about which chains are pooled,
// because a chain the writer thinks is per-payer and the reader thinks is
// pooled is a deposit matched against an identity nobody issued.
func Pooled(chain string) bool {
	return chainFamily[cryptopaymentintent.Chain(strings.ToLower(strings.TrimSpace(chain)))] == FamilyXRPL
}

// Fold normalises a chain-native identifier — a deposit address, a transaction
// id — for comparison and for keying.
//
// It is ONE function rather than one per kind of identifier because it encodes
// one fact: whether that chain's textual encoding is case-significant.
//
//	EVM     hex, case-INsensitive. The custody service hands back EIP-55
//	        checksummed addresses and node logs are lowercase; comparing them
//	        literally would miss every deposit, so both sides are lowercased.
//	Solana  base58, case-SIGNIFICANT. `aB…` and `Ab…` are different accounts and
//	        different signatures, so nothing is folded. Lowercasing here would
//	        merge distinct addresses into one map entry — and while the collision
//	        is vanishingly unlikely, "unlikely" is not a property to hang a
//	        custody address on when exactness is free.
//	TON     base64url, case-SIGNIFICANT. Same reasoning as Solana.
//	XRPL    base58 (a DIFFERENT alphabet from Solana's, same property) and, for
//	        a currency code, case-significant too: XRPL compares codes as 160-bit
//	        values, so "usd" and "USD" are two tokens.
//
// The non-EVM families all leave identifiers alone, which is only safe because
// their READERS canonicalise before returning: tonrpc and xrplrpc both render a
// transaction id as lowercase hex whatever the endpoint answered, and both echo
// back the caller's own spelling of a watched address rather than the node's.
// That keeps the dedup key a function of the EVENT and not of an endpoint's
// rendering choice.
//
// A chain added to chainFamily without a considered answer here gets the EVM
// fold, which is the safe default in the only direction that matters: folding
// too much makes two addresses ambiguous, which fails the pass closed, while
// folding too little makes a real deposit invisible.
func (a Asset) Fold(s string) string {
	s = strings.TrimSpace(s)
	switch a.Family() {
	case FamilySolana, FamilyTON, FamilyXRPL:
		return s
	default:
		return strings.ToLower(s)
	}
}

// Identity is WHAT NAMES THE INTENT a transfer belongs to on this chain.
//
// It exists because "the deposit address" is not that thing everywhere, and
// bending the address field until it was would have been the bug: on XRPL every
// payer shares ONE custody account (see Pooled) and only the destination tag
// says whose money arrived, so matching on the address alone would credit ten
// thousand customers' deposits to whichever intent the map happened to keep.
//
//	EVM / Solana / TON  the folded address. One payer, one address.
//	XRPL                the folded address AND the destination tag.
//
// Both sides of every comparison — the minted intent and the observed transfer
// — go through this one function, so the two halves of the match cannot drift.
// The separator cannot occur in any chain's address or tag encoding, so no
// (address, tag) pair can be spelled two ways or collide with another.
func (a Asset) Identity(address, tag string) string {
	addr := a.Fold(address)
	if !a.Pooled() {
		return addr
	}
	return addr + "#" + strings.TrimSpace(tag)
}

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
//	ETH / MATIC / AVAX / BNB / SOL — a native coin cannot be priced, and on the
//	  EVM it cannot even be OBSERVED here (a native transfer emits no log, so
//	  eth_getLogs cannot see it). Not watched at all.
//	BTC — needs a price feed. Not this rail. It is the chain that genuinely
//	  cannot be added without an oracle, and adding one to credit it would put
//	  a guess about a customer's money on this path.
//	XRP / TON — native coins, same problem as ETH: unpriceable here.
//	Anything else pegged — add it here WITH its peg, deliberately.
//
// Reaching a new chain therefore needs a Reader and NOT a price oracle, which
// is precisely why TON and XRPL could be added: TON carries jetton USDT and
// XRPL carries issued USDC and RLUSD, all at a dollar.
//
// The known, bounded, deliberate risk this accepts: a depegged stablecoin is
// credited above its market value. That is the standard bargain every payment
// rail makes for stablecoins, and it is stated here rather than hidden in a
// rate table nobody reads.
var pegCents = map[string]int64{
	"usdc": 100,
	"usdt": 100,
	// Ripple USD, the XRPL-native dollar-pegged stablecoin. Added with its peg
	// deliberately, exactly as the paragraph above requires.
	"rlusd": 100,
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
//	CRYPTO_DEPOSIT_RPC_<CHAIN>            read endpoint for that chain
//	CRYPTO_DEPOSIT_TOKEN_<CHAIN>_<TOKEN>  what defines that token there — an
//	                                      ERC-20 contract, an SPL mint, a jetton
//	                                      master, or <CURRENCY>.<ISSUER>
//
// e.g. CRYPTO_DEPOSIT_RPC_BASE + CRYPTO_DEPOSIT_TOKEN_BASE_USDC,
// CRYPTO_DEPOSIT_RPC_SOLANA + CRYPTO_DEPOSIT_TOKEN_SOLANA_USDC,
// CRYPTO_DEPOSIT_RPC_TON + CRYPTO_DEPOSIT_TOKEN_TON_USDT, or
// CRYPTO_DEPOSIT_RPC_XRPL + CRYPTO_DEPOSIT_TOKEN_XRPL_RLUSD.
//
// "Endpoint" is deliberately looser than "JSON-RPC URL": the EVM, Solana and
// XRPL are read over JSON-RPC, while TON is read over the TON Index HTTP API,
// because TON has no per-account transaction index in its node RPC at all. One
// key per chain either way.
//
// This mirrors util/husd exactly, including the part that matters: a token
// address has NO DEFAULT. An unset deploy watches nothing instead of guessing at
// an address, and a token's address is per-chain anyway (USDC on BSC is a
// different contract with different decimals than USDC on Ethereum), so a
// built-in table would be a list of constants nobody can verify from here.
// A third key configures the POOLED custody account, for the chains that have
// one:
//
//	CRYPTO_DEPOSIT_ADDRESS_<CHAIN>  the one account every payer sends to
//
// e.g. CRYPTO_DEPOSIT_ADDRESS_XRPL=rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De. It has
// no default and no fallback for the same reason a token address does not: an
// address is where money goes, and inventing one is inventing a destination.
const (
	envRPCPrefix     = "CRYPTO_DEPOSIT_RPC_"
	envTokenPrefix   = "CRYPTO_DEPOSIT_TOKEN_"
	envAddressPrefix = "CRYPTO_DEPOSIT_ADDRESS_"
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
	pooledAddr := map[string]string{}
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
			// The token ADDRESS is not lowercased here. Its case may be
			// significant (base58 on Solana), so it is normalised by the chain's
			// own fold once the chain is known, below.
			toks = append(toks, tok{
				chain:    strings.ToLower(rest[:us]),
				token:    strings.ToLower(rest[us+1:]),
				contract: val,
			})
		case strings.HasPrefix(key, envAddressPrefix):
			// NOT lowercased: an r-address is base58 and case-significant. It is
			// normalised, like a token address, by the chain's own fold below.
			pooledAddr[strings.ToLower(key[len(envAddressPrefix):])] = val
		case strings.HasPrefix(key, envRPCPrefix):
			rpc[strings.ToLower(key[len(envRPCPrefix):])] = val
		}
	}

	// A pooled address on a chain that mints per payer is a misunderstanding
	// with a very bad ending — every payer handed the same address on a chain
	// where the address is the ONLY thing that says whose money it is — so it
	// is refused here rather than ignored. Refusing an unknown chain too keeps
	// a typo'd variable from silently configuring nothing.
	for chain := range pooledAddr {
		if _, ok := chainFamily[cryptopaymentintent.Chain(chain)]; !ok {
			return nil, fmt.Errorf("depositwatch: %s%s configures chain %q, which this rail has no reader for; the readable chains are %s",
				envAddressPrefix, strings.ToUpper(chain), chain, strings.Join(readableChains(), ", "))
		}
		if !Pooled(chain) {
			return nil, fmt.Errorf("depositwatch: %s%s sets a shared deposit address on chain %q, which mints one address per payer — a shared address there would hand every payer the same destination and lose track of whose money is whose",
				envAddressPrefix, strings.ToUpper(chain), chain)
		}
	}

	assets := make([]Asset, 0, len(toks))
	seen := map[string]bool{}
	for _, t := range toks {
		if _, ok := pegCents[t.token]; !ok {
			return nil, fmt.Errorf("depositwatch: %s%s_%s configures token %q, which has no USD peg — this rail credits only dollar-pegged tokens (%s); remove it or add its peg deliberately",
				envTokenPrefix, strings.ToUpper(t.chain), strings.ToUpper(t.token), t.token, strings.Join(creditableTokens(), ", "))
		}
		if _, ok := chainFamily[cryptopaymentintent.Chain(t.chain)]; !ok {
			return nil, fmt.Errorf("depositwatch: %s%s_%s configures chain %q, which this rail has no reader for — a chain nobody can read is money nobody is watching; the readable chains are %s",
				envTokenPrefix, strings.ToUpper(t.chain), strings.ToUpper(t.token), t.chain, strings.Join(readableChains(), ", "))
		}
		a := Asset{Chain: t.chain, Token: t.token}
		a.Contract = a.Fold(t.contract)
		if err := a.validateContract(); err != nil {
			return nil, fmt.Errorf("depositwatch: %s%s_%s %w",
				envTokenPrefix, strings.ToUpper(t.chain), strings.ToUpper(t.token), err)
		}
		url, ok := rpc[t.chain]
		if !ok {
			return nil, fmt.Errorf("depositwatch: token %s is configured on chain %q but %s%s is unset — a watched token with no endpoint is money nobody is watching",
				t.token, t.chain, envRPCPrefix, strings.ToUpper(t.chain))
		}
		a.RPCURL = url
		// A pooled chain with no configured account is money with nowhere to
		// go: the mint side has no address to hand out and no way to invent
		// one. That is an incoherent table exactly like a token with no
		// endpoint, so it fails the same way — at boot, loudly — rather than
		// at the first customer who picks XRPL.
		if a.Pooled() {
			addr, ok := pooledAddr[t.chain]
			if !ok {
				return nil, fmt.Errorf("depositwatch: token %s is configured on chain %q, which shares ONE deposit account across payers, but %s%s is unset — there is no address to hand out and none may be invented",
					t.token, t.chain, envAddressPrefix, strings.ToUpper(t.chain))
			}
			a.PooledAddress = a.Fold(addr)
			if err := a.validatePooledAddress(); err != nil {
				return nil, fmt.Errorf("depositwatch: %s%s %w", envAddressPrefix, strings.ToUpper(t.chain), err)
			}
		}
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

// validateContract checks the configured token address is written the way its
// chain writes one.
//
// This is a SHAPE check and nothing more — it cannot tell USDC's mint from any
// other 32-byte account. What proves the address is the token it is labelled as
// is the chain read in Watcher.verify, which asks the token itself. This one
// exists so an address pasted from the wrong chain entirely fails at boot,
// loudly, instead of at the first deposit.
func (a Asset) validateContract() error {
	switch a.Family() {
	case FamilySolana:
		if !isBase58Account(a.Contract) {
			return fmt.Errorf("is not a base58 32-byte SPL mint address: %q", a.Contract)
		}
	case FamilyTON:
		if !isTONAddress(a.Contract) {
			return fmt.Errorf("is not a TON jetton master address — expected the 48-character user-friendly form (EQ…/UQ…) or raw <workchain>:<64 hex>: %q", a.Contract)
		}
	case FamilyXRPL:
		// The full parse lives in billing/xrplrpc, which also checksums the
		// issuer; this is the cheap boundary check that keeps a bare address or
		// a bare currency code out of a slot that needs both.
		if !isXRPLIssued(a.Contract) {
			return fmt.Errorf("is not an XRPL issued token — expected <CURRENCY>.<ISSUER>, e.g. RLUSD.rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De: %q", a.Contract)
		}
	default:
		if !isHexAddress(a.Contract) {
			return fmt.Errorf("is not a 20-byte hex address: %q", a.Contract)
		}
	}
	return nil
}

// validatePooledAddress checks the configured custody account is written the
// way its chain writes an ACCOUNT — which is not how it writes a token.
//
// The same SHAPE-check reasoning as validateContract, and the same limit: this
// cannot tell our account from anyone else's. What it stops is the paste that
// would otherwise be discovered by a customer — the issuer half of a
// <CURRENCY>.<ISSUER> pair, an EVM address, an empty string — landing in the
// slot that decides where money is sent.
func (a Asset) validatePooledAddress() error {
	switch a.Family() {
	case FamilyXRPL:
		if !isXRPLAddress(a.PooledAddress) {
			return fmt.Errorf("is not an XRPL account address — expected the classic r-address form, e.g. rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De: %q", a.PooledAddress)
		}
	default:
		// Unreachable: only pooled chains get here, and XRPL is the only one.
		// It is a refusal rather than a nil so that pooling a second chain
		// fails here instead of accepting whatever that chain calls an address.
		return fmt.Errorf("chain %q is pooled but has no address form defined here", a.Chain)
	}
	return nil
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

// readableChains lists the chains this rail has a reader for, sorted.
func readableChains() []string {
	out := make([]string, 0, len(chainFamily))
	for c := range chainFamily {
		out = append(out, string(c))
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

// base58Alphabet is Bitcoin/Solana base58: no 0, O, I or l, so the characters a
// human is most likely to confuse cannot both be valid.
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// isBase58Account reports whether s could be a 32-byte Solana address.
//
// The length is checked in CHARACTERS because base58 carries no length of its
// own: 32 bytes encodes to 32–44 characters depending on how many leading zero
// bytes it has. Decoding it properly is billing/solanarpc's job and it does
// refuse anything that is not exactly 32 bytes; this is the cheap boundary
// check that keeps an EVM address out of a Solana slot at boot.
func isBase58Account(s string) bool {
	if len(s) < 32 || len(s) > 44 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune(base58Alphabet, r) {
			return false
		}
	}
	return true
}

// isTONAddress reports whether s could be a TON account address, in either of
// the two forms TON publishes: the 48-character user-friendly base64url form
// (36 bytes: flags, workchain, 32-byte hash, CRC-16) that every explorer and
// token page shows, or the raw <workchain>:<64 hex> form the indexer answers
// in. Both are accepted because both are pasted, and billing/tonrpc parses both
// with the same function — including the CRC that catches a typo.
func isTONAddress(s string) bool {
	if wc, hash, ok := strings.Cut(s, ":"); ok {
		if wc == "" || len(hash) != 64 {
			return false
		}
		if wc[0] == '-' {
			wc = wc[1:]
		}
		return wc != "" && allASCIIDigits(wc) && isHex(hash)
	}
	if len(s) != 48 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-' || r == '_' || r == '+' || r == '/' || r == '=') {
			return false
		}
	}
	return true
}

// isXRPLIssued reports whether s could be an XRPL <CURRENCY>.<ISSUER> pair: a
// 3-character or 40-hex currency code, and a classic r-address.
func isXRPLIssued(s string) bool {
	cur, issuer, ok := strings.Cut(s, ".")
	if !ok {
		return false
	}
	// A currency code is 3 ASCII characters, a ticker of up to 20, or the raw
	// 40-hex form. billing/xrplrpc turns all three into the same 160 bits.
	if cur == "" || len(cur) > 40 || (len(cur) > 20 && !(len(cur) == 40 && isHex(cur))) {
		return false
	}
	return isXRPLAddress(issuer)
}

// isXRPLAddress reports whether s could be an XRPL classic account address: an
// r-prefixed base58 string in XRPL's own alphabet, 25–35 characters.
//
// It is ONE function because an XRPL account is an XRPL account whether it
// appears as a token's issuer or as the custody account deposits are sent to.
// Two copies of this rule would be two chances for the issuer half of a token
// to be accepted somewhere the account half is not.
func isXRPLAddress(s string) bool {
	if len(s) < 25 || len(s) > 35 || s[0] != 'r' {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune(rippleAlphabet, r) {
			return false
		}
	}
	return true
}

// rippleAlphabet is XRPL's base58 alphabet — the same 58 characters as
// Bitcoin's in a DIFFERENT order, which is why an XRPL address must never be
// checked against base58Alphabet above.
const rippleAlphabet = "rpshnaf39wBUDNEGHJKLM4PQRST7VWXYZ2bcdeCg65jkm8oFqi1tuvAxyz"

func isHex(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return len(s) > 0
}

func allASCIIDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
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
