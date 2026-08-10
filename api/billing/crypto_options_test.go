package billing

import (
	"encoding/json"
	"testing"

	"github.com/hanzoai/commerce/billing/depositwatch"
)

// The picker must never name an asset nothing can credit.
//
// This endpoint used to answer from the MPC processor, which knows only whether
// it can DERIVE an address on a chain — it says yes to nine. Nothing followed
// any of those addresses to a balance, so the list walked buyers toward assets
// the system could receive and never credit: the same defect that stopped the
// rail (cryptoDepositsCanBeCredited), one endpoint upstream of it.
//
// These test offeredFrom rather than the handler, and that is deliberate. A
// first attempt drove GetCryptoOptions end to end and PASSED against a mutant
// that advertised a hardcoded list — because the custody-signer probe fails in
// a test environment, so the handler answered 503 and the assertion was never
// reached. The rule lives in the projection; test it where it lives, and where
// it cannot be silently skipped.
func TestOfferedFrom(t *testing.T) {
	t.Run("no assets offers nothing", func(t *testing.T) {
		// The whole point. An unconfigured watcher must yield an empty menu,
		// never the full one — advertising is what invites money.
		chains, tokens := offeredFrom(nil, allChains)
		if len(chains) != 0 || len(tokens) != 0 {
			t.Fatalf("nothing is watched, yet the picker offers chains=%v tokens=%v", chains, tokens)
		}
	})

	t.Run("offers exactly what is watched, deduplicated and ordered", func(t *testing.T) {
		chains, tokens := offeredFrom([]depositwatch.Asset{
			{Chain: "ethereum", Token: "usdc"},
			{Chain: "base", Token: "usdc"}, // same token, second chain
			{Chain: "base", Token: "usdt"}, // same chain, second token
		}, allChains)
		if got, want := chains, []string{"base", "ethereum"}; !eq(got, want) {
			t.Fatalf("chains = %v, want %v", got, want)
		}
		if got, want := tokens, []string{"usdc", "usdt"}; !eq(got, want) {
			t.Fatalf("tokens = %v, want %v", got, want)
		}
	})

	t.Run("normalises case, so one asset cannot appear twice", func(t *testing.T) {
		chains, tokens := offeredFrom([]depositwatch.Asset{
			{Chain: "Base", Token: "USDC"},
			{Chain: "base", Token: "usdc"},
		}, allChains)
		if len(chains) != 1 || len(tokens) != 1 {
			t.Fatalf("case variants split one asset in two: chains=%v tokens=%v", chains, tokens)
		}
	})

	t.Run("empty marshals to a list, never null", func(t *testing.T) {
		// A picker handed `null` where it expects an array is a client-side
		// crash, which is a worse failure than an empty menu.
		chains, tokens := offeredFrom(nil, allChains)
		b, err := json.Marshal(map[string]any{"chains": chains, "tokens": tokens})
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != `{"chains":[],"tokens":[]}` {
			t.Fatalf("marshalled to %s, want empty arrays", b)
		}
	})
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// allChains is a custody signer that can mint anywhere, so the tests above
// isolate the watched-assets half of the rule.
var allChains = []string{"ethereum", "base", "polygon", "arbitrum", "optimism", "avalanche", "bsc", "lux", "zoo", "solana", "bitcoin"}

// The other half of the rule, and the one this rail can hit TODAY.
//
// avalanche is readable — depositwatch.chainFamily gives it an EVM reader, so a
// deposit there would be seen and credited — and it is not mintable, because the
// signer's alias table has no entry for the name and would refuse every
// signature over the coins. Offering it would put an Avalanche button in front of
// a buyer that dead-ends after they have chosen an amount, which is the ORIGINAL
// defect of this endpoint wearing the opposite mask.
func TestOfferedFrom_OnlyChainsAnAddressCanBeMintedOn(t *testing.T) {
	watched := []depositwatch.Asset{
		{Chain: "base", Token: "usdc"},
		{Chain: "avalanche", Token: "usdc"},
	}
	// The real MPC chain list: the EVM chains the signer knows by name, and
	// deliberately not avalanche or zoo.
	mintable := []string{"bitcoin", "ethereum", "polygon", "arbitrum", "optimism", "base", "lux", "bsc", "solana", "ton"}

	chains, tokens := offeredFrom(watched, mintable)
	if got, want := chains, []string{"base"}; !eq(got, want) {
		t.Fatalf("chains = %v, want %v — a chain with no mintable address must not be offered", got, want)
	}
	if got, want := tokens, []string{"usdc"}; !eq(got, want) {
		t.Fatalf("tokens = %v, want %v", got, want)
	}

	// And a token that ONLY exists on an unmintable chain disappears with it,
	// rather than lingering in the picker with nowhere to send it.
	only := []depositwatch.Asset{{Chain: "avalanche", Token: "usdt"}}
	chains, tokens = offeredFrom(only, mintable)
	if len(chains) != 0 || len(tokens) != 0 {
		t.Fatalf("an unmintable chain still offers chains=%v tokens=%v", chains, tokens)
	}
}

// A signer that can mint nowhere offers nothing, exactly as a watcher that
// watches nothing does. Both inputs fail closed.
func TestOfferedFrom_NoMintableChainsOffersNothing(t *testing.T) {
	chains, tokens := offeredFrom([]depositwatch.Asset{{Chain: "base", Token: "usdc"}}, nil)
	if len(chains) != 0 || len(tokens) != 0 {
		t.Fatalf("a signer that can mint nowhere still offers chains=%v tokens=%v", chains, tokens)
	}
}
