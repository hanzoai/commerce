package depositwatch

import (
	"strings"
	"testing"
)

// The POOLED CUSTODY ACCOUNT — the write side's half of the model the read side
// already implements.
//
// A pooled chain has one account and a per-deposit tag, so the account is a
// CONFIGURED fact rather than a minted one. Everything here is about the config
// table refusing to describe a rail that would take money it cannot route:
// there is no default account, no invented account, and no account on a chain
// that does not pool.

const (
	// The shared custody account, in XRPL's own base58 alphabet.
	poolAccount = "rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De"
	xrplRPC     = "CRYPTO_DEPOSIT_RPC_XRPL=https://xrpl.rpc"
	xrplRLUSDTk = "CRYPTO_DEPOSIT_TOKEN_XRPL_RLUSD=RLUSD.rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De"
	xrplUSDCTk  = "CRYPTO_DEPOSIT_TOKEN_XRPL_USDC=USDC.rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De"
)

// A pooled chain configured with its account: every token on that chain carries
// it, because the ACCOUNT is per-chain even though an asset is per-token.
func TestPooledAddress_IsCarriedByEveryTokenOnTheChain(t *testing.T) {
	assets, err := AssetsFromEnv([]string{
		xrplRPC, xrplRLUSDTk, xrplUSDCTk,
		"CRYPTO_DEPOSIT_ADDRESS_XRPL=" + poolAccount,
	})
	if err != nil {
		t.Fatalf("AssetsFromEnv: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("parsed %d assets, want 2: %+v", len(assets), assets)
	}
	for _, a := range assets {
		if !a.Pooled() {
			t.Fatalf("%s is not pooled", a.Key())
		}
		if a.PooledAddress != poolAccount {
			t.Fatalf("%s carries pooled address %q, want %q", a.Key(), a.PooledAddress, poolAccount)
		}
	}
}

// THE fail-closed rule for the mint side: a pooled chain with no configured
// account must not produce an asset at all.
//
// The whole point is that "asset exists" can then MEAN "there is an address to
// hand out", so the mint path has no unconfigured case to get wrong. If this
// were merely an empty field, the rail would offer XRPL in the picker and hand
// the customer an empty string.
func TestPooledAddress_AChainWithNoAccountIsRefused(t *testing.T) {
	_, err := AssetsFromEnv([]string{xrplRPC, xrplRLUSDTk})
	if err == nil {
		t.Fatal("a pooled chain with no configured custody account was accepted — the rail would offer XRPL with nowhere to send")
	}
	if !strings.Contains(err.Error(), "CRYPTO_DEPOSIT_ADDRESS_XRPL") || !strings.Contains(err.Error(), "is unset") {
		// Not pedantry about wording: "unset" and "malformed" are different
		// operator actions, and an operator told their address is the wrong
		// SHAPE will go looking for the address they never set.
		t.Fatalf("the error does not say the variable is unset: %v", err)
	}
}

// THE INVARIANT the mint path is built on: every pooled asset that EXISTS has
// an account to be paid to.
//
// This is what lets api/billing treat "no address" as impossible rather than as
// a case to handle, and it is what keeps the picker honest — offeredFrom offers
// a pooled chain on the strength of it being watched, so an asset with an empty
// address would put XRPL in front of a buyer and then 503 after they chose an
// amount. Stated over the OUTPUT rather than over any one branch, so it holds
// however the parsing is later rearranged.
func TestPooledAddress_EveryPooledAssetHasAnAccount(t *testing.T) {
	for _, environ := range [][]string{
		{xrplRPC, xrplRLUSDTk, "CRYPTO_DEPOSIT_ADDRESS_XRPL=" + poolAccount},
		{xrplRPC, xrplRLUSDTk, xrplUSDCTk, "CRYPTO_DEPOSIT_ADDRESS_XRPL=" + poolAccount},
		{xrplRPC, xrplRLUSDTk}, // refused; the loop below then runs over nothing
		{xrplRPC, xrplRLUSDTk, "CRYPTO_DEPOSIT_ADDRESS_XRPL="},
		{xrplRPC, xrplRLUSDTk, "CRYPTO_DEPOSIT_ADDRESS_XRPL=   "},
		{
			"CRYPTO_DEPOSIT_RPC_BASE=https://base.rpc",
			"CRYPTO_DEPOSIT_TOKEN_BASE_USDC=0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
			xrplRPC, xrplRLUSDTk, "CRYPTO_DEPOSIT_ADDRESS_XRPL=" + poolAccount,
		},
	} {
		assets, err := AssetsFromEnv(environ)
		if err != nil {
			continue // refused outright, which satisfies the invariant vacuously
		}
		for _, a := range assets {
			if a.Pooled() && a.PooledAddress == "" {
				t.Fatalf("asset %s is pooled but has no custody account, from %v — it would be offered in the picker with nowhere to send", a.Key(), environ)
			}
			if !a.Pooled() && a.PooledAddress != "" {
				t.Fatalf("per-payer asset %s carries pooled address %q", a.Key(), a.PooledAddress)
			}
		}
	}
}

// The inverse, and the more dangerous one: a shared account on a chain that
// mints per payer would hand EVERY payer the same address on a chain where the
// address is the only thing that says whose money it is.
func TestPooledAddress_OnAPerPayerChainIsRefused(t *testing.T) {
	_, err := AssetsFromEnv([]string{
		"CRYPTO_DEPOSIT_RPC_BASE=https://base.rpc",
		"CRYPTO_DEPOSIT_TOKEN_BASE_USDC=0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
		"CRYPTO_DEPOSIT_ADDRESS_BASE=0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
	})
	if err == nil {
		t.Fatal("a shared deposit address was accepted on a per-payer chain — every payer would be handed the same destination")
	}
	if !strings.Contains(err.Error(), "one address per payer") {
		t.Fatalf("the error does not explain the problem: %v", err)
	}
}

// A per-payer chain never carries a pooled address, so nothing downstream can
// mistake one for the other.
func TestPooledAddress_IsEmptyOnAPerPayerChain(t *testing.T) {
	assets, err := AssetsFromEnv([]string{
		"CRYPTO_DEPOSIT_RPC_BASE=https://base.rpc",
		"CRYPTO_DEPOSIT_TOKEN_BASE_USDC=0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
	})
	if err != nil {
		t.Fatalf("AssetsFromEnv: %v", err)
	}
	if assets[0].Pooled() {
		t.Fatal("base is pooled")
	}
	if assets[0].PooledAddress != "" {
		t.Fatalf("a per-payer chain carries a pooled address: %q", assets[0].PooledAddress)
	}
}

// Shape checks on the account itself. Each of these is a paste somebody makes,
// and the alternative to catching it here is catching it at the first customer.
func TestPooledAddress_RefusesWhatIsNotAnAccount(t *testing.T) {
	for _, tc := range []struct {
		name, address, wantIn string
	}{{
		// The most likely paste of all: the token config is
		// <CURRENCY>.<ISSUER>, so the whole pair lands in the address slot.
		name:    "an issued-token pair rather than an account",
		address: "RLUSD." + poolAccount,
		wantIn:  "classic r-address",
	}, {
		name:    "an EVM address",
		address: "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
		wantIn:  "classic r-address",
	}, {
		// Bitcoin/Solana base58 is a DIFFERENT alphabet ordering, and `0` is
		// not in XRPL's at all.
		name:    "base58 from the wrong alphabet",
		address: "r0OIl" + poolAccount[5:],
		wantIn:  "classic r-address",
	}, {
		name:    "truncated",
		address: poolAccount[:12],
		wantIn:  "classic r-address",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AssetsFromEnv([]string{
				xrplRPC, xrplRLUSDTk,
				"CRYPTO_DEPOSIT_ADDRESS_XRPL=" + tc.address,
			})
			if err == nil {
				t.Fatalf("%q was accepted as a custody account", tc.address)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("error %q does not contain %q", err, tc.wantIn)
			}
		})
	}
}

// An address on a chain nothing can read is a typo, not a configuration.
func TestPooledAddress_OnAnUnreadableChainIsRefused(t *testing.T) {
	_, err := AssetsFromEnv([]string{"CRYPTO_DEPOSIT_ADDRESS_RIPPLE=" + poolAccount})
	if err == nil {
		t.Fatal("a custody account was accepted on a chain this rail cannot read")
	}
}

// Pooled(chain) and Asset.Pooled() must be ONE fact. The read side matches
// deposits by identity and the write side decides which door a request takes;
// if they could disagree, an intent would be issued in a shape the watcher
// never looks for.
func TestPooled_ChainAndAssetAgree(t *testing.T) {
	for chain, family := range chainFamily {
		want := family == FamilyXRPL
		if got := Pooled(string(chain)); got != want {
			t.Fatalf("Pooled(%q) = %v, want %v", chain, got, want)
		}
		if got := (Asset{Chain: string(chain)}).Pooled(); got != want {
			t.Fatalf("Asset{%q}.Pooled() = %v, want %v", chain, got, want)
		}
	}
	// The mint side receives a chain from a request body, so the free function
	// must normalise the way the rest of the rail does.
	if !Pooled(" XRPL ") {
		t.Fatal("Pooled does not normalise its input, so a request could take the per-payer door on a pooled chain")
	}
	if Pooled("") || Pooled("dogecoin") {
		t.Fatal("an unknown chain reports as pooled")
	}
}
