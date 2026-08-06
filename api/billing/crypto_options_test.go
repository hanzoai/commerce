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
		chains, tokens := offeredFrom(nil)
		if len(chains) != 0 || len(tokens) != 0 {
			t.Fatalf("nothing is watched, yet the picker offers chains=%v tokens=%v", chains, tokens)
		}
	})

	t.Run("offers exactly what is watched, deduplicated and ordered", func(t *testing.T) {
		chains, tokens := offeredFrom([]depositwatch.Asset{
			{Chain: "ethereum", Token: "usdc"},
			{Chain: "base", Token: "usdc"}, // same token, second chain
			{Chain: "base", Token: "usdt"}, // same chain, second token
		})
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
		})
		if len(chains) != 1 || len(tokens) != 1 {
			t.Fatalf("case variants split one asset in two: chains=%v tokens=%v", chains, tokens)
		}
	})

	t.Run("empty marshals to a list, never null", func(t *testing.T) {
		// A picker handed `null` where it expects an array is a client-side
		// crash, which is a worse failure than an empty menu.
		chains, tokens := offeredFrom(nil)
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
