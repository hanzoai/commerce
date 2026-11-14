package fx

import (
	"context"
	"math"
	"testing"
)

func stub(t *testing.T, r map[string]float64) {
	old := Rate
	Rate = func(_ context.Context, c string) (float64, error) { return r[c], nil }
	t.Cleanup(func() { Rate = old })
}

// P_local = ceilToIncrement(P_usd x FX). 2000 USD cents is $20; the answer is in the
// currency's own minor unit, rounded up to the increment, and the credit stays 2000.
func TestThePriceIsTheConvertedAmountRoundedUpToTheIncrement(t *testing.T) {
	stub(t, map[string]float64{"inr": 94.25, "jpy": 150.4, "krw": 1403.2, "cny": 7.21, "eur": 0.9137, "kwd": 0.30712, "huf": 360, "isk": 140})
	for _, c := range []struct {
		cur  string
		step int64 // minor units
		want int64
		dec  int
	}{
		{"inr", 100, 188500, 2}, // 1885.00 -> Rs 1885, whole rupee
		{"jpy", 10, 3010, 0},    // 3008 -> 3010, ten yen
		{"krw", 100, 28100, 0},  // 28064 -> 28100, a hundred won
		{"cny", 100, 14500, 2},  // 144.20 -> 145 yuan
		{"eur", 100, 1900, 2},   // 18.274 -> 19 euro
		{"kwd", 10, 6150, 3},    // 6.1424 -> 6.15 dinar, ends in 0
		{"huf", 100, 720000, 2}, // 7200.00
		{"isk", 100, 280000, 2}, // 2800 kr in hundredths
		{"krw", 1, 28064, 0},    // rounding off: exact, rounded up to a won
		{"inr", 1, 188500, 2},   // already whole
	} {
		want := c.want
		got, err := Convert(2000, c.cur, rateOf(t, c.cur), c.step)
		if err != nil || got != want {
			t.Errorf("%s step %d: %d (%v), want %d", c.cur, c.step, got, err, want)
		}
		if got%c.step != 0 {
			t.Errorf("%s: %d is not a multiple of %d", c.cur, got, c.step)
		}
		// The buyer always covers the credit: the amount, at the rate, is at least $20.
		if paid := float64(got) / math.Pow10(c.dec) / rateOf(t, c.cur) * 100; paid < 2000-1e-6 {
			t.Errorf("%s: %d buys only %.4f usd cents", c.cur, got, paid)
		}
	}
}

func rateOf(t *testing.T, cur string) float64 {
	r, err := Rate(context.Background(), cur)
	if err != nil || r == 0 {
		t.Fatalf("no stub rate for %s", cur)
	}
	return r
}

// Go: 8 USD at 94.25 is exactly 754 rupees, with no margin and no fee added.
func TestWithNoMarginTheBuyerPaysExactlyTheConvertedAmount(t *testing.T) {
	got, err := Convert(800, "inr", 94.25, 100)
	if err != nil || got != 75400 {
		t.Fatalf("800 usd cents at 94.25 = %d (%v), want 75400 paise (Rs 754)", got, err)
	}
	// Rs 94.25 x 8.01 = 754.9425 rounds up to Rs 755: the increment, and nothing else.
	if got, _ := Convert(801, "inr", 94.25, 100); got != 75500 {
		t.Fatalf("801 usd cents at 94.25 = %d, want 75500", got)
	}
}

func TestStepMustBeSomethingStripeTakes(t *testing.T) {
	for _, c := range []struct {
		cur  string
		step int64
		ok   bool
	}{{"kwd", 10, true}, {"kwd", 5, false}, {"isk", 100, true}, {"isk", 50, false}, {"jpy", 1, true}, {"usd", 1, true}, {"inr", 0, false}} {
		_, err := Convert(2000, c.cur, 1, c.step)
		if (err == nil) != c.ok {
			t.Errorf("%s step %d: %v", c.cur, c.step, err)
		}
	}
	for cur, want := range map[string]int64{"usd": 1, "jpy": 1, "huf": 1, "twd": 1, "isk": 100, "ugx": 100, "kwd": 10, "bhd": 10, "jod": 10, "omr": 10, "tnd": 10} {
		if got := Granule(cur); got != want {
			t.Errorf("Granule(%s) = %d, want %d", cur, got, want)
		}
	}
}

func TestMarketQuoteReportsTheRateAppliedAndNeverShortsTheCredit(t *testing.T) {
	stub(t, map[string]float64{"jpy": 149.37, "kwd": 0.30711, "ugx": 3712.3, "inr": 83.3333})
	for _, cur := range []string{"jpy", "kwd", "ugx", "inr"} {
		step := Granule(cur) * 10
		for cents := int64(50); cents < 30000; cents += 137 {
			got, rate, err := Quote(context.Background(), cents, cur, step)
			if err != nil {
				t.Fatal(err)
			}
			dec := map[string]int{"jpy": 0, "kwd": 3, "ugx": 2, "inr": 2}[cur]
			if got%step != 0 {
				t.Fatalf("%s %d is not a multiple of %d", cur, got, step)
			}
			if paid := float64(got) / math.Pow10(dec) / rate * 100; paid < float64(cents)-1e-6 {
				t.Fatalf("%s %d cents quoted %d, which buys only %.4f", cur, cents, got, paid)
			}
		}
	}
}

func TestAChargeOutsideWhatStripeTakesIsRefused(t *testing.T) {
	if _, err := Convert(1<<40, "vnd", 25000, 1); err == nil {
		t.Fatal("an amount past 12 digits must be refused")
	}
}
