package billing

import (
	"testing"

	"github.com/hanzoai/commerce/billing/depositwatch"
)

// An address is handed out only for an asset something is watching.
//
// This replaced `const cryptoDepositsCanBeCredited = false`. That constant was
// right for what it knew — when written, nothing credited ANY chain, so one
// false covered every case. Flipping it to true once the watcher existed would
// have re-created the original defect for every asset the watcher is not
// configured for: a real custody address, minted, that nothing will ever look
// at. "Crediting exists" is not "crediting exists FOR THIS CHAIN", and the gap
// between those two sentences is where the money went.
//
// So the tests are about the GAP, not about the happy path.
func TestCreditable(t *testing.T) {
	watched := []depositwatch.Asset{
		{Chain: "base", Token: "usdc"},
		{Chain: "ethereum", Token: "usdt"},
	}

	t.Run("a watched asset is creditable", func(t *testing.T) {
		if !creditable(watched, "base", "usdc") {
			t.Fatal("base/usdc is watched and must be creditable")
		}
	})

	t.Run("nothing watched means nothing creditable", func(t *testing.T) {
		// The default state of any deployment. It must be closed, not open.
		for _, a := range watched {
			if creditable(nil, a.Chain, a.Token) {
				t.Fatalf("%s/%s creditable with an unconfigured watcher", a.Chain, a.Token)
			}
		}
	})

	t.Run("a watched CHAIN does not make its other tokens creditable", func(t *testing.T) {
		// The precise shape of the bug a blanket flag would have reintroduced:
		// base is watched, but only for usdc. Nothing scans base/usdt, so an
		// address handed out for it collects money no pass will ever see.
		if creditable(watched, "base", "usdt") {
			t.Fatal("base/usdt creditable though only base/usdc is watched")
		}
	})

	t.Run("a watched TOKEN does not make its other chains creditable", func(t *testing.T) {
		// The mirror image, and just as lossy: usdc is watched on base, which
		// says nothing about usdc on polygon.
		if creditable(watched, "polygon", "usdc") {
			t.Fatal("polygon/usdc creditable though only base/usdc is watched")
		}
	})

	t.Run("case and padding do not open the gate", func(t *testing.T) {
		// A request body is caller-supplied. If "BASE" missed the watched set it
		// would be refused — safe — but if it matched a DIFFERENT asset it would
		// not be, so the comparison must be exactly as strict as the one the
		// picker uses.
		if !creditable(watched, "BASE", "USDC") {
			t.Error("case variation refused a genuinely watched asset")
		}
		if creditable(watched, "base", "usdc-fake") {
			t.Error("a token that merely shares a prefix was accepted")
		}
	})
}

// The gate and the picker must answer from the same set, or a buyer is shown an
// asset the mint path refuses — this endpoint's original defect, wearing its
// third mask.
func TestOfferedAndCreditableAgree(t *testing.T) {
	watched := []depositwatch.Asset{
		{Chain: "base", Token: "usdc"},
		{Chain: "solana", Token: "usdc"},
	}
	mintable := []string{"base", "ethereum"} // no solana: custody cannot mint it

	chains, _ := offeredFrom(watched, mintable)
	for _, ch := range chains {
		if !creditable(watched, ch, "usdc") {
			t.Fatalf("picker offers %s/usdc but the mint path would refuse it", ch)
		}
	}
	// And the converse: solana is creditable but unmintable, so it must not be
	// offered — otherwise the buyer picks an amount and then meets a refusal.
	if len(chains) != 1 || chains[0] != "base" {
		t.Fatalf("offered %v, want only [base]", chains)
	}
}
