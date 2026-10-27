package billing

import (
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The pairs are the menu itself: a chain and a token each on offer are not a pair
// on offer.
func TestOfferedAssetsAreThePairs(t *testing.T) {
	got := offeredAssets([]depositwatch.Asset{
		{Chain: "Base", Token: "USDC"},
		{Chain: "base", Token: "usdc"}, // a case variant is the same pair
		{Chain: "ethereum", Token: "usdt"},
		{Chain: "solana", Token: "usdc"}, // nothing can mint a solana address
	}, []string{"base", "ethereum"})
	want := []CryptoAsset{{Chain: "base", Token: "usdc"}, {Chain: "ethereum", Token: "usdt"}}
	if len(got) != len(want) {
		t.Fatalf("pairs = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pairs = %+v, want %+v", got, want)
		}
	}
	// base/usdt is on both axes and is not a pair the rail accepts.
	chains, tokens := axesOf(got)
	if !eq(chains, []string{"base", "ethereum"}) || !eq(tokens, []string{"usdc", "usdt"}) {
		t.Fatalf("axes = %v / %v", chains, tokens)
	}

	empty, err := menu(offeredAssets(nil, nil), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"chains":[],"tokens":[],"assets":[],"gateway":[]}` {
		t.Fatalf("an empty menu marshalled to %s", b)
	}
}

// minimumCents is the watcher's own arithmetic read backwards: a deposit of the
// minimum credits, one cent less credits nothing.
func TestMinimumIsWhatTheWatcherCredits(t *testing.T) {
	cents := func(c int64) *big.Int { return new(big.Int).Mul(big.NewInt(c), big.NewInt(10_000)) } // 6 decimals
	const peg = 100 * depositwatch.RateScale
	for _, terms := range []depositwatch.Terms{
		{},
		{FeeCents: 50},
		{FeeCents: 50, SlippageBps: 150},
		{SlippageBps: 9_999},
	} {
		min := minimumCents(terms)
		if got, err := depositwatch.AmountCents(cents(min), 6, peg, terms); err != nil || got < 1 {
			t.Errorf("%+v: the minimum %d credits %d (%v); want at least a cent", terms, min, got, err)
		}
		_, err := depositwatch.AmountCents(cents(min-1), 6, peg, terms)
		if !errors.Is(err, depositwatch.ErrDust) && !errors.Is(err, depositwatch.ErrUnderFee) {
			t.Errorf("%+v: one cent under the minimum %d credited (%v)", terms, min, err)
		}
	}
	if got := minimumCents(depositwatch.Terms{}); got != 1 {
		t.Fatalf("with nothing deducted the minimum is %d cents, want 1", got)
	}
}

// A deposit tells a payer what they need to pay it: the amount they named, the
// smallest deposit that credits, what is deducted, how deep the chain must go, how
// deep it is, and what was credited.
func TestTheDepositCarriesItsTerms(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	installWatcher(t, append(baseUSDCEnv(), "CRYPTO_DEPOSIT_FEE_BASE=25"), true)

	in := cryptopaymentintent.New(orgCtx("acme"))
	in.Amount = 500
	in.Chain = cryptopaymentintent.Base
	in.Token = "usdc"
	in.DepositAddress = "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	in.Status = cryptopaymentintent.Confirming
	in.Confirmations = 3

	got := toCryptoDepositResponse(in)
	if got.AmountCents != 500 || got.FeeCents != 25 || got.MinimumCents != 26 {
		t.Fatalf("amount %d fee %d minimum %d; want 500, 25, 26", got.AmountCents, got.FeeCents, got.MinimumCents)
	}
	if got.RequiredConfirmations != 20 || got.Confirmations != 3 || got.CreditedCents != 0 {
		t.Fatalf("required %d seen %d credited %d; want 20, 3, 0", got.RequiredConfirmations, got.Confirmations, got.CreditedCents)
	}

	in.Status = cryptopaymentintent.Succeeded
	in.SettlementAmount = 475
	if got := toCryptoDepositResponse(in); got.CreditedCents != 475 {
		t.Fatalf("credited %d after success, want 475", got.CreditedCents)
	}

	// Nothing watching the asset: no minimum to state, because nothing credits.
	in.Chain = cryptopaymentintent.Polygon
	if got := toCryptoDepositResponse(in); got.MinimumCents != 0 || got.FeeCents != 0 {
		t.Fatalf("an unwatched asset states minimum %d fee %d", got.MinimumCents, got.FeeCents)
	}
}
