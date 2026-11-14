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

// 2000 USD cents is $20. The quote is in each currency's own minor unit and rounds up
// to what Stripe takes; the credit stays 2000 exactly.
func TestAQuoteIsInTheCurrencysOwnMinorUnit(t *testing.T) {
	stub(t, map[string]float64{"jpy": 150, "krw": 1400, "inr": 83.5, "kwd": 0.307, "huf": 360, "isk": 140, "usd": 1})
	for _, c := range []struct {
		cur  string
		want int64
		dec  int
	}{
		{"jpy", 3000, 0},   // 20 * 150 yen
		{"krw", 28000, 0},  // 20 * 1400 won
		{"inr", 167000, 2}, // 20 * 83.50 rupees, in paise
		{"kwd", 6140, 3},   // 6.140 dinars: fils, ending in 0
		{"huf", 720000, 2}, // 7200.00 forint, charged in hundredths
		{"isk", 280000, 2}, // 2800 kronur stated in hundredths, fraction 00
	} {
		got, rate, err := Quote(context.Background(), 2000, c.cur, 0)
		if err != nil || got != c.want {
			t.Errorf("%s: %d (%v), want %d", c.cur, got, err, c.want)
		}
		// Round trip: the disclosed amount in major units, at the disclosed rate, is the
		// dollars credited, to within one Step of rounding up and never below.
		major := float64(got) / math.Pow10(c.dec)
		back := major / rate * 100
		if back < 2000-1e-6 || back > 2000+float64(Step(c.cur))/math.Pow10(c.dec)/rate*100+1e-6 {
			t.Errorf("%s: %d pays for %.4f usd cents, want 2000", c.cur, got, back)
		}
	}
}

func TestRoundingNeverShortsTheBuyersCredit(t *testing.T) {
	stub(t, map[string]float64{"jpy": 149.37, "kwd": 0.30711, "ugx": 3712.3, "inr": 83.3333})
	for _, cur := range []string{"jpy", "kwd", "ugx", "inr"} {
		for cents := int64(50); cents < 30000; cents += 137 {
			got, rate, err := Quote(context.Background(), cents, cur, 200)
			if err != nil {
				t.Fatal(err)
			}
			if got%Step(cur) != 0 {
				t.Fatalf("%s %d is not a multiple of %d", cur, got, Step(cur))
			}
			dec := map[string]int{"jpy": 0, "kwd": 3, "ugx": 2, "inr": 2}[cur]
			if paid := float64(got) / math.Pow10(dec) / rate * 100; paid < float64(cents)-1e-6 {
				t.Fatalf("%s %d cents quoted %d, which buys only %.4f", cur, cents, got, paid)
			}
		}
	}
}

func TestStepFollowsStripesRules(t *testing.T) {
	for cur, want := range map[string]int64{"usd": 1, "jpy": 1, "krw": 1, "vnd": 1, "huf": 1, "twd": 1, "isk": 100, "ugx": 100, "kwd": 10, "bhd": 10, "jod": 10, "omr": 10, "tnd": 10} {
		if got := Step(cur); got != want {
			t.Errorf("Step(%s) = %d, want %d", cur, got, want)
		}
	}
}

func TestAChargeOutsideWhatStripeTakesIsRefused(t *testing.T) {
	stub(t, map[string]float64{"vnd": 25000})
	if _, _, err := Quote(context.Background(), 1<<40, "vnd", 0); err == nil {
		t.Fatal("an amount past 12 digits must be refused")
	}
}
