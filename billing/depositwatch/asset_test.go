package depositwatch

import (
	"strings"
	"testing"
)

func TestAssetsFromEnv(t *testing.T) {
	assets, err := AssetsFromEnv([]string{
		"PATH=/usr/bin",
		"CRYPTO_DEPOSIT_RPC_ETHEREUM=https://eth.rpc",
		"CRYPTO_DEPOSIT_RPC_BASE=https://base.rpc",
		"CRYPTO_DEPOSIT_TOKEN_ETHEREUM_USDC=0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		"CRYPTO_DEPOSIT_TOKEN_ETHEREUM_USDT=0xdAC17F958D2ee523a2206206994597C13D831ec7",
		"CRYPTO_DEPOSIT_TOKEN_BASE_USDC=0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
	})
	if err != nil {
		t.Fatalf("AssetsFromEnv: %v", err)
	}
	if len(assets) != 3 {
		t.Fatalf("parsed %d assets, want 3: %+v", len(assets), assets)
	}
	// Sorted by key, so a boot log and a status read always agree.
	if got := assets[0].Key(); got != "base:usdc" {
		t.Fatalf("assets are not in a stable order: first is %q", got)
	}
	eth := assets[1]
	if eth.Key() != "ethereum:usdc" {
		t.Fatalf("second asset is %q", eth.Key())
	}
	// The contract is lowercased so it matches what the chain reports.
	if eth.Contract != "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48" {
		t.Fatalf("contract not normalised: %q", eth.Contract)
	}
	if eth.RPCURL != "https://eth.rpc" {
		t.Fatalf("asset bound to the wrong endpoint: %q", eth.RPCURL)
	}
	if eth.PegCents() != 100 || eth.PegRate() != "1.00" {
		t.Fatalf("usdc peg = %d (%q), want 100 (1.00)", eth.PegCents(), eth.PegRate())
	}
}

// An empty environment watches nothing. It must never fall back to a built-in
// contract address: a default address is a constant nobody in this process can
// verify, and pointing a money watcher at the wrong contract is unrecoverable.
func TestAssetsFromEnv_UnsetWatchesNothing(t *testing.T) {
	assets, err := AssetsFromEnv([]string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatalf("AssetsFromEnv: %v", err)
	}
	if len(assets) != 0 {
		t.Fatalf("an unconfigured deploy watches %d assets, want 0: %+v", len(assets), assets)
	}
}

func TestAssetsFromEnv_RefusesIncoherentConfig(t *testing.T) {
	for _, tc := range []struct {
		name    string
		environ []string
		wantIn  string
	}{
		{
			name: "token with no endpoint",
			environ: []string{
				"CRYPTO_DEPOSIT_TOKEN_POLYGON_USDC=0x3c499c542cEF5E3811e1192ce70d8cC03d5c3359",
			},
			wantIn: "CRYPTO_DEPOSIT_RPC_POLYGON",
		},
		{
			name: "token this rail cannot price",
			environ: []string{
				"CRYPTO_DEPOSIT_RPC_ETHEREUM=https://eth.rpc",
				"CRYPTO_DEPOSIT_TOKEN_ETHEREUM_WETH=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
			},
			wantIn: "no USD peg",
		},
		{
			name: "contract that is not an address",
			environ: []string{
				"CRYPTO_DEPOSIT_RPC_ETHEREUM=https://eth.rpc",
				"CRYPTO_DEPOSIT_TOKEN_ETHEREUM_USDC=usdc.eth",
			},
			wantIn: "hex address",
		},
		{
			name: "malformed key",
			environ: []string{
				"CRYPTO_DEPOSIT_RPC_ETHEREUM=https://eth.rpc",
				"CRYPTO_DEPOSIT_TOKEN_ETHEREUM=0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
			},
			wantIn: "<CHAIN>_<TOKEN>",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assets, err := AssetsFromEnv(tc.environ)
			if err == nil {
				t.Fatalf("accepted an incoherent config, producing %+v", assets)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("error %q does not say what to fix (want it to mention %q)", err, tc.wantIn)
			}
		})
	}
}

// The peg table IS the creditable set: a token with no declared peg cannot be
// configured at all, so no volatile asset can be credited by mistake.
func TestPegTable_OnlyDollarPeggedTokens(t *testing.T) {
	for _, volatile := range []string{"eth", "matic", "avax", "bnb", "btc", "lux", "zoo", "sol"} {
		if _, ok := pegCents[volatile]; ok {
			t.Fatalf("%q has a peg — this rail has no price oracle and must not credit a volatile asset", volatile)
		}
	}
	for tok, peg := range pegCents {
		if peg != 100 {
			t.Fatalf("%q is pegged at %d cents; a token in this table must be worth one dollar", tok, peg)
		}
	}
}
