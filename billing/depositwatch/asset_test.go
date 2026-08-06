package depositwatch

import (
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/cryptopaymentintent"
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

// ── non-EVM chains ──────────────────────────────────────────────────────────

const solUSDCMint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

// A Solana asset configures exactly like an EVM one — same two variables, same
// failure modes — and the ONE thing that must differ does: its mint address is
// base58 and survives configuration byte for byte.
func TestAssetsFromEnv_Solana(t *testing.T) {
	assets, err := AssetsFromEnv([]string{
		"CRYPTO_DEPOSIT_RPC_SOLANA=https://api.mainnet-beta.solana.com",
		"CRYPTO_DEPOSIT_TOKEN_SOLANA_USDC=" + solUSDCMint,
	})
	if err != nil {
		t.Fatalf("AssetsFromEnv: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("parsed %d assets, want 1: %+v", len(assets), assets)
	}
	a := assets[0]
	if a.Key() != "solana:usdc" {
		t.Fatalf("asset key = %q", a.Key())
	}
	// THE assertion. Lowercasing a base58 mint yields an address that decodes to
	// different bytes — a mint that does not exist — and the watcher would then
	// refuse every deposit or, worse, read some other account.
	if a.Contract != solUSDCMint {
		t.Fatalf("mint was normalised to %q, want %q — base58 is case-significant", a.Contract, solUSDCMint)
	}
	if !a.IsSolana() {
		t.Fatal("a solana asset is not reported as one; it would be handed an EVM reader")
	}
	if a.PegCents() != 100 {
		t.Fatalf("solana usdc peg = %d, want 100", a.PegCents())
	}
}

// An address from the wrong chain is caught at BOOT, where it is a typo, rather
// than at the first deposit, where it is somebody's money.
func TestAssetsFromEnv_RefusesAnAddressFromTheWrongChain(t *testing.T) {
	for _, tc := range []struct {
		name    string
		environ []string
		wantIn  string
	}{
		{
			name: "an EVM contract configured on solana",
			environ: []string{
				"CRYPTO_DEPOSIT_RPC_SOLANA=https://sol.rpc",
				"CRYPTO_DEPOSIT_TOKEN_SOLANA_USDC=0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
			},
			wantIn: "base58 32-byte SPL mint",
		},
		{
			name: "a solana mint configured on an EVM chain",
			environ: []string{
				"CRYPTO_DEPOSIT_RPC_BASE=https://base.rpc",
				"CRYPTO_DEPOSIT_TOKEN_BASE_USDC=" + solUSDCMint,
			},
			wantIn: "20-byte hex address",
		},
		{
			// A chain with no Reader would otherwise be handed the EVM client on
			// the assumption that everything is an EVM — it would error every 30
			// seconds and watch nothing while looking configured.
			name: "a chain this rail cannot read",
			environ: []string{
				"CRYPTO_DEPOSIT_RPC_TON=https://ton.rpc",
				"CRYPTO_DEPOSIT_TOKEN_TON_USDT=EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs",
			},
			wantIn: "no reader for",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assets, err := AssetsFromEnv(tc.environ)
			if err == nil {
				t.Fatalf("accepted %+v", assets)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("error %q does not mention %q", err, tc.wantIn)
			}
		})
	}
}

// Fold encodes one fact per chain: whether its textual addresses are
// case-significant. Getting it backwards either merges two customers' addresses
// or makes every deposit invisible.
func TestFold_IsPerChain(t *testing.T) {
	evm := Asset{Chain: "base", Token: "usdc"}
	if got := evm.Fold(" 0xAbCd "); got != "0xabcd" {
		t.Fatalf("EVM fold = %q, want 0xabcd — a checksummed address must match a lowercase log", got)
	}
	sol := Asset{Chain: "solana", Token: "usdc"}
	if got := sol.Fold(" " + solUSDCMint + " "); got != solUSDCMint {
		t.Fatalf("solana fold = %q, want %q unchanged", got, solUSDCMint)
	}
	// Two base58 addresses that differ only in case are DIFFERENT accounts.
	if sol.Fold("aBc") == sol.Fold("AbC") {
		t.Fatal("the solana fold merges distinct base58 addresses")
	}
	// A chain nobody has classified gets the EVM fold, which fails closed
	// (ambiguous) rather than blind (invisible).
	unknown := Asset{Chain: "made-up", Token: "usdc"}
	if got := unknown.Fold("0xAB"); got != "0xab" {
		t.Fatalf("unclassified chain fold = %q, want the EVM fold", got)
	}
}

// Every chain the intent model can mint an address for must be classified here,
// or configuring it fails at boot with no way to fix it but a code change.
func TestChainFamily_CoversEveryChainTheIntentModelKnows(t *testing.T) {
	for _, c := range []cryptopaymentintent.Chain{
		cryptopaymentintent.Ethereum, cryptopaymentintent.Solana, cryptopaymentintent.Base,
		cryptopaymentintent.Polygon, cryptopaymentintent.Arbitrum, cryptopaymentintent.Optimism,
		cryptopaymentintent.Avalanche, cryptopaymentintent.BSC, cryptopaymentintent.Lux,
		cryptopaymentintent.Zoo,
	} {
		if _, ok := chainFamily[c]; !ok {
			t.Fatalf("chain %q can be minted but has no reader family — configuring it would fail at boot", c)
		}
	}
}
