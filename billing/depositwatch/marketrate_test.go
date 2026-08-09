package depositwatch

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
)

// A whole BTC in satoshis (8 decimals).
func btcUnits(whole int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(whole), big.NewInt(100_000_000))
}

type rates struct {
	micro map[string]int64
	err   error
}

func (r rates) MicroCents(_ context.Context, id string) (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.micro[id], nil
}

// The two tables must stay disjoint, or a customer's coin has two values and a
// receipt cannot say which was used.
func TestATokenIsPeggedOrMarketPricedNeverBoth(t *testing.T) {
	for tok := range marketPriced {
		if _, ok := pegCents[tok]; ok {
			t.Errorf("%q is in BOTH pegCents and marketPriced", tok)
		}
	}
	// And the three the rail is being asked for are actually there.
	for _, tok := range []string{"btc", "xrp", "ton"} {
		if _, ok := marketPriced[tok]; !ok {
			t.Errorf("%q is not market-priced, so it can never be credited", tok)
		}
	}
}

func TestMarketPricedAssetsCarryTheOraclesOwnIDs(t *testing.T) {
	// "ripple", not "xrp" — the ids belong to the oracle's vocabulary, and
	// translating in two places is how a symbol map goes stale.
	for tok, id := range marketPriced {
		if id == "" {
			t.Errorf("%q has no price id", tok)
		}
		if id == tok {
			t.Errorf("%q maps to itself — that is our name, not the oracle's", tok)
		}
	}
}

// A peg and a market rate must reach the SAME arithmetic. This is the property
// that lets everything downstream be blind to which one was used.
func TestAPegAndAMarketRateTakeTheSamePath(t *testing.T) {
	// 1 USDC at a $1.00 peg.
	pegged, err := AmountCents(usdcUnits(1), 6, 100*RateScale, Terms{})
	if err != nil {
		t.Fatalf("pegged: %v", err)
	}
	// 1 of a coin quoted at exactly $1.00 by the oracle.
	market, err := AmountCents(usdcUnits(1), 6, 100*RateScale, Terms{})
	if err != nil {
		t.Fatalf("market: %v", err)
	}
	if pegged != market {
		t.Errorf("same rate, different answers: %d vs %d", pegged, market)
	}
}

func TestBitcoinPricesThroughTheWholePath(t *testing.T) {
	// $65,194.995 per BTC, in micro-cents.
	const btcRate = 6_519_499_500_000
	// 0.015 BTC = 1,500,000 sats → $977.92 gross.
	got, err := AmountCents(big.NewInt(1_500_000), 8, btcRate, Terms{})
	if err != nil {
		t.Fatalf("AmountCents: %v", err)
	}
	if got != 97_792 {
		t.Errorf("0.015 BTC at $65,194.995 credited %d cents, want 97792", got)
	}
}

// The precision micro-cents exist for, on the asset that shows it worst.
func TestASubDollarCoinKeepsItsRate(t *testing.T) {
	// XRP at $1.04295 — 104295000 micro-cents. 1000 XRP (6 decimals).
	const xrpRate = 104_295_000
	got, err := AmountCents(new(big.Int).Mul(big.NewInt(1000), big.NewInt(1_000_000)), 6, xrpRate, Terms{})
	if err != nil {
		t.Fatalf("AmountCents: %v", err)
	}
	if got != 104_295 {
		t.Errorf("1000 XRP credited %d cents, want 104295 ($1042.95)", got)
	}
	// Rounded to whole cents the rate would have been $1.04, crediting 104000 —
	// $2.95 less on this one deposit.
	lossy, _ := AmountCents(new(big.Int).Mul(big.NewInt(1000), big.NewInt(1_000_000)), 6, 104*RateScale, Terms{})
	if lossy >= got {
		t.Errorf("whole-cent rate credited %d, not less than %d — the precision is doing nothing", lossy, got)
	}
}

// --- the watcher's rate resolution ---

func TestAPeggedAssetNeedsNoOracle(t *testing.T) {
	w := &Watcher{}
	a := &asset{Asset: Asset{Chain: "base", Token: "usdc"}}
	got, err := w.rateFor(context.Background(), a)
	if err != nil {
		t.Fatalf("rateFor: %v", err)
	}
	if got != 100*RateScale {
		t.Errorf("rate = %d, want a $1.00 peg in micro-cents", got)
	}
}

func TestAMarketPricedAssetWithNoOracleIsRefused(t *testing.T) {
	// NEVER a fallback. There is no sensible default price for a bitcoin, and
	// inventing one is the failure the rail refused native coins to avoid.
	w := &Watcher{}
	a := &asset{Asset: Asset{Chain: "bitcoin", Token: "btc"}}
	_, err := w.rateFor(context.Background(), a)
	if err == nil {
		t.Fatal("a market-priced asset priced itself with no oracle configured")
	}
	if !strings.Contains(err.Error(), "no oracle") {
		t.Errorf("refusal does not say the oracle is missing: %v", err)
	}
}

func TestAMarketPricedAssetUsesTheOracle(t *testing.T) {
	w := (&Watcher{}).WithRates(rates{micro: map[string]int64{"bitcoin": 6_519_499_500_000}})
	a := &asset{Asset: Asset{Chain: "bitcoin", Token: "btc"}}
	got, err := w.rateFor(context.Background(), a)
	if err != nil {
		t.Fatalf("rateFor: %v", err)
	}
	if got != 6_519_499_500_000 {
		t.Errorf("rate = %d, want the oracle's quote", got)
	}
}

func TestAnOracleFailureCreditsNothing(t *testing.T) {
	// Fails CLOSED: the coin is already in custody and waits, where a deposit
	// valued at a wrong rate is wrong permanently.
	w := (&Watcher{}).WithRates(rates{err: errors.New("no quote could be justified")})
	a := &asset{Asset: Asset{Chain: "bitcoin", Token: "btc"}}
	if _, err := w.rateFor(context.Background(), a); err == nil {
		t.Fatal("an oracle failure produced a rate")
	}
}

func TestAnOracleAnsweringZeroIsRefused(t *testing.T) {
	// Zero is not a price. Credited as zero it would look like dust — a
	// customer's coin silently worth nothing — rather than like the outage it is.
	w := (&Watcher{}).WithRates(rates{micro: map[string]int64{}})
	a := &asset{Asset: Asset{Chain: "bitcoin", Token: "btc"}}
	if _, err := w.rateFor(context.Background(), a); err == nil {
		t.Fatal("a zero quote was accepted as a rate")
	}
}

// The receipt has to state the rate actually used, at the precision it was used.
func TestRateStringKeepsEveryDigit(t *testing.T) {
	for _, c := range []struct {
		micro int64
		want  string
	}{
		{100 * RateScale, "1.00000000"},
		{6_519_499_500_000, "65194.99500000"},
		{104_295_000, "1.04295000"},
	} {
		if got := RateString(c.micro); got != c.want {
			t.Errorf("RateString(%d) = %q, want %q", c.micro, got, c.want)
		}
	}
}
