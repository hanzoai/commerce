package depositwatch

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
)

// One whole USDC, 6 decimals, $1.00 peg — 100 cents gross before any deduction.
func usdcUnits(whole int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(whole), big.NewInt(1_000_000))
}

// THE PROPERTY THAT MATTERS MOST: terms nobody configured change nothing.
//
// Every existing deployment runs with zero Terms, so if this drifts the rail
// starts quietly deducting from deposits it never deducted from before.
func TestZeroTermsCreditTheFullGross(t *testing.T) {
	for _, whole := range []int64{1, 7, 1000} {
		got, err := AmountCents(usdcUnits(whole), 6, 100*RateScale, Terms{})
		if err != nil {
			t.Fatalf("%d USDC: %v", whole, err)
		}
		if want := whole * 100; got != want {
			t.Errorf("%d USDC credited %d cents, want %d — zero terms deducted something", whole, got, want)
		}
	}
}

func TestFeeIsDeductedInWholeCents(t *testing.T) {
	// $100 in, a $2.50 sweep fee, $97.50 credited.
	got, err := AmountCents(usdcUnits(100), 6, 100*RateScale, Terms{FeeCents: 250})
	if err != nil {
		t.Fatalf("AmountCents: %v", err)
	}
	if got != 9_750 {
		t.Errorf("credited %d cents, want 9750", got)
	}
}

func TestSlippageIsAProportion(t *testing.T) {
	// 100 bps = 1%. $100 in, $99 credited.
	got, err := AmountCents(usdcUnits(100), 6, 100*RateScale, Terms{SlippageBps: 100})
	if err != nil {
		t.Fatalf("AmountCents: %v", err)
	}
	if got != 9_900 {
		t.Errorf("credited %d cents, want 9900", got)
	}
}

func TestSlippageThenFee(t *testing.T) {
	// Order is load-bearing: the haircut is proportional and the fee is flat, so
	// fee-then-haircut would also shave the fee and quietly under-charge it.
	// $100 → 1% → $99.00 → minus $2.50 → $96.50.
	got, err := AmountCents(usdcUnits(100), 6, 100*RateScale, Terms{SlippageBps: 100, FeeCents: 250})
	if err != nil {
		t.Fatalf("AmountCents: %v", err)
	}
	if got != 9_650 {
		t.Errorf("credited %d cents, want 9650", got)
	}
}

// The deductions must round DOWN, like everything else on this path, and must
// round once rather than three times.
func TestDeductionsTruncateDownOnce(t *testing.T) {
	// 1 USDC at 1% is 99.00 cents exactly; 3 USDC at 33 bps is 299.01 → 299.
	got, err := AmountCents(usdcUnits(3), 6, 100*RateScale, Terms{SlippageBps: 33})
	if err != nil {
		t.Fatalf("AmountCents: %v", err)
	}
	if got != 299 {
		t.Errorf("credited %d cents, want 299 (299.01 truncated down)", got)
	}
}

func TestATransferUnderTheFeeIsRefusedDistinctlyFromDust(t *testing.T) {
	// $1 in, $2.50 fee. Real money arrived and it does not cover the cost of
	// moving it — a different thing from dust, and it needs a different word:
	// this customer sent something worth telling them about.
	_, err := AmountCents(usdcUnits(1), 6, 100*RateScale, Terms{FeeCents: 250})
	if !errors.Is(err, ErrUnderFee) {
		t.Fatalf("err = %v, want ErrUnderFee", err)
	}
	if errors.Is(err, ErrDust) {
		t.Error("an under-fee transfer reports as dust — the two must stay distinguishable")
	}
	// The refusal has to say what arrived and what the fee was, or nobody can
	// answer the customer asking where their deposit went.
	if !strings.Contains(err.Error(), "100") || !strings.Contains(err.Error(), "250") {
		t.Errorf("refusal names neither the amount nor the fee: %v", err)
	}
}

func TestExactlyTheFeeCreditsNothing(t *testing.T) {
	// Net zero is not a credit. Writing a zero-cent ledger row would record a
	// deposit that moved no balance.
	if _, err := AmountCents(usdcUnits(1), 6, 100*RateScale, Terms{FeeCents: 100}); !errors.Is(err, ErrUnderFee) {
		t.Error("a transfer worth exactly the fee credited something")
	}
}

// Terms that cannot be meant must be refused, because every one of them is
// silent once the arithmetic runs.
func TestNonsensicalTermsAreRefused(t *testing.T) {
	for _, c := range []struct {
		name  string
		terms Terms
	}{
		{"negative fee credits more than arrived", Terms{FeeCents: -1}},
		{"negative slippage credits more than arrived", Terms{SlippageBps: -1}},
		{"100% haircut credits nothing however much is sent", Terms{SlippageBps: 10_000}},
		{"over 100% is not a haircut at all", Terms{SlippageBps: 20_000}},
	} {
		if _, err := AmountCents(usdcUnits(100), 6, 100, c.terms); err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}
}

// --- multi-tenancy: the org's terms override the platform default ---

type resolver struct {
	terms Terms
	ok    bool
	err   error
}

func (r resolver) TermsFor(context.Context, string, string) (Terms, bool, error) {
	return r.terms, r.ok, r.err
}

func TestNoResolverMeansEveryOrgIsOnTheDefault(t *testing.T) {
	def := Terms{FeeCents: 250}
	got, err := resolveTerms(context.Background(), nil, "acme", "ethereum", def)
	if err != nil {
		t.Fatalf("resolveTerms: %v", err)
	}
	if got != def {
		t.Errorf("terms = %+v, want the platform default %+v", got, def)
	}
}

func TestAnOrgWithNoOpinionGetsTheDefault(t *testing.T) {
	def := Terms{FeeCents: 250}
	got, err := resolveTerms(context.Background(), resolver{ok: false}, "acme", "ethereum", def)
	if err != nil {
		t.Fatalf("resolveTerms: %v", err)
	}
	if got != def {
		t.Errorf("terms = %+v, want the default when the resolver has no opinion", got)
	}
}

func TestAnOrgNegotiatedToNothingIsNotTheSameAsUnconfigured(t *testing.T) {
	// THE distinction ok=false exists for. An org that pays nothing must not be
	// silently put back on the platform's fee.
	def := Terms{FeeCents: 250}
	got, err := resolveTerms(context.Background(), resolver{terms: Terms{}, ok: true}, "bigco", "ethereum", def)
	if err != nil {
		t.Fatalf("resolveTerms: %v", err)
	}
	if got.Deducts() {
		t.Errorf("terms = %+v, want nothing deducted — the org's zero was overwritten by the default", got)
	}
}

func TestAnOrgOverrideReplacesTheDefaultEntirely(t *testing.T) {
	// The override is not merged field-by-field. Merging would mean an org that
	// negotiated a lower fee silently keeps the platform's slippage, which is a
	// term nobody agreed to.
	def := Terms{FeeCents: 250, SlippageBps: 50}
	own := Terms{FeeCents: 10}
	got, err := resolveTerms(context.Background(), resolver{terms: own, ok: true}, "bigco", "ethereum", def)
	if err != nil {
		t.Fatalf("resolveTerms: %v", err)
	}
	if got != own {
		t.Errorf("terms = %+v, want exactly the org's %+v", got, own)
	}
}

func TestAResolverErrorFailsClosed(t *testing.T) {
	// It must NOT fall back to the platform default. The default is usually the
	// cheaper one, so falling back would credit an org on terms it is not on and
	// the error that caused it would be invisible. Nothing credits this pass.
	def := Terms{FeeCents: 250}
	_, err := resolveTerms(context.Background(), resolver{err: errors.New("store down")}, "acme", "ethereum", def)
	if err == nil {
		t.Fatal("a resolver failure fell back to the platform default")
	}
	if !strings.Contains(err.Error(), "acme") || !strings.Contains(err.Error(), "ethereum") {
		t.Errorf("error names neither the org nor the chain: %v", err)
	}
}
