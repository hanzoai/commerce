package price

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Rates are in MICRO-CENTS (cents × Scale). $65,181.72 is 6_518_172 cents is
// 6_518_172_000_000 micro-cents; the constants below are written that way so a
// reader can see the magnitude rather than count zeros.
const (
	btcLow  = 6_518_172_000_000 // $65,181.72
	btcHigh = 6_518_320_000_000 // $65,183.20
)

// stub is a Source that answers whatever it was told to, so the ORACLE's
// judgement is what these tests exercise — not an exchange's uptime.
type stub struct {
	name  string
	micro int64
	err   error
}

func (s stub) Name() string { return s.name }
func (s stub) MicroCents(context.Context, string) (int64, error) {
	return s.micro, s.err
}

func quote(t *testing.T, sources ...Source) (Quote, error) {
	t.Helper()
	o, err := New(sources...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return o.Quote(context.Background(), "BTC")
}

// --- what must be REFUSED. Each of these is a wrong credit if it returns a number.

func TestOneSourceIsNeverEnough(t *testing.T) {
	// A single venue is one outage and one bad tick away from pricing every
	// deposit in the estate, and the arithmetic downstream cannot tell a wrong
	// number from a right one.
	if _, err := New(stub{name: "only", micro: btcLow}); err == nil {
		t.Fatal("New accepted a single source — one venue can price the whole rail alone")
	}
}

func TestASourceThatFailsDoesNotCount(t *testing.T) {
	_, err := quote(t,
		stub{name: "a", micro: btcLow},
		stub{name: "b", err: errors.New("timeout")},
	)
	if !errors.Is(err, ErrNoQuote) {
		t.Fatalf("err = %v, want ErrNoQuote — one live source is not a quorum", err)
	}
}

func TestAZeroAnswerIsNotAPrice(t *testing.T) {
	// The trap: a parse that yields 0 must be a FAILURE, not a price of nothing.
	// Credited as zero it would look like dust — a customer's coin silently worth
	// nothing — instead of like the outage it is.
	_, err := quote(t,
		stub{name: "a", micro: btcLow},
		stub{name: "b", micro: 0},
	)
	if !errors.Is(err, ErrNoQuote) {
		t.Fatalf("err = %v, want ErrNoQuote — a zero answer counted as a source", err)
	}
}

func TestDisagreementIsRefusedRatherThanResolved(t *testing.T) {
	// 10% apart on a liquid pair means one of them is wrong and there is no way
	// to tell which. Picking the "better" one is the guess this package exists
	// to avoid.
	const tenPercentUp = btcLow + btcLow/10
	_, err := quote(t,
		stub{name: "a", micro: btcLow},
		stub{name: "b", micro: tenPercentUp},
	)
	if !errors.Is(err, ErrNoQuote) {
		t.Fatalf("err = %v, want ErrNoQuote", err)
	}
	if !strings.Contains(err.Error(), "disagree") {
		t.Errorf("refusal does not say the sources disagreed: %v", err)
	}
	// The refusal must name the numbers, or an operator cannot tell a stale feed
	// from a real move.
	if !strings.Contains(err.Error(), "a=") || !strings.Contains(err.Error(), "b=") {
		t.Errorf("refusal does not name both sources and their quotes: %v", err)
	}
}

func TestTwoNamesForOneVenueIsRefused(t *testing.T) {
	// Independence is the property MinSources buys. Two entries for one exchange
	// satisfy the COUNT while providing one opinion.
	if _, err := New(stub{name: "kraken", micro: btcLow}, stub{name: "Kraken ", micro: btcLow}); err == nil {
		t.Fatal("New accepted the same venue twice")
	}
}

// --- what must be ANSWERED, and with which number.

func TestAgreeingSourcesQuoteTheLower(t *testing.T) {
	q, err := quote(t,
		stub{name: "a", micro: btcLow},
		stub{name: "b", micro: btcHigh},
	)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	// The LOWER of two, deliberately: the same downward bias AmountCents uses so
	// the rail can never credit value that was not sent.
	if q.MicroCents != btcLow {
		t.Errorf("micro-cents = %d, want the lower of the two (%d)", q.MicroCents, btcLow)
	}
	if len(q.Sources) != 2 {
		t.Errorf("sources = %v, want both named for the audit trail", q.Sources)
	}
	if q.At.IsZero() {
		t.Error("quote carries no timestamp — a market credit must say when it was priced")
	}
}

func TestThreeSourcesTakeTheMiddle(t *testing.T) {
	q, err := quote(t,
		stub{name: "a", micro: 6_500_000_000_000},
		stub{name: "b", micro: 6_510_000_000_000},
		stub{name: "c", micro: 6_520_000_000_000},
	)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	// The median rejects one outlier without needing to know which one is wrong.
	if q.MicroCents != 6_510_000_000_000 {
		t.Errorf("micro-cents = %d, want the median 6510000000000", q.MicroCents)
	}
}

func TestAnOutlierWithinSpreadDoesNotMoveTheMedian(t *testing.T) {
	q, err := quote(t,
		stub{name: "a", micro: 6_500_000_000_000},
		stub{name: "b", micro: 6_501_000_000_000},
		stub{name: "c", micro: 6_600_000_000_000}, // 1.5% high — inside MaxSpread
	)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if q.MicroCents != 6_501_000_000_000 {
		t.Errorf("micro-cents = %d, want 6501000000000 — the outlier moved the median", q.MicroCents)
	}
}

// The precision that made micro-cents necessary: a sub-dollar coin must not be
// flattened to whole cents. XRP at $1.04295 credited as $1.04 under-pays 0.28%
// on every deposit — $2.80 on a $1,000 top-up, silently, forever.
func TestSubDollarRatesKeepTheirPrecision(t *testing.T) {
	const xrp = 104_295_000 // $1.04295
	q, err := quote(t,
		stub{name: "a", micro: xrp},
		stub{name: "b", micro: xrp},
	)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if q.MicroCents != xrp {
		t.Fatalf("micro-cents = %d, want %d", q.MicroCents, xrp)
	}
	// Whole cents would have been 104. Anything that round-trips to 104 cents
	// exactly has lost the precision this scale exists for.
	if q.MicroCents/Scale != 104 || q.MicroCents%Scale == 0 {
		t.Errorf("rate %d lost its sub-cent digits", q.MicroCents)
	}
}

// --- the decimal parse, which is where a float would quietly lose money.

func TestDecimalToMicroCentsIsExact(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int64 // 0 means "must error"
	}{
		{"65181.725", 6_518_172_500_000}, // BTC, every digit kept
		{"1.04295", 104_295_000},         // XRP — the case whole cents destroyed
		{"1.3412", 134_120_000},          // TON
		{"0.99", 99_000_000},
		{"100", 10_000_000_000},
		{"100.", 10_000_000_000},
		{"0.005", 500_000},   // half a cent is now REPRESENTABLE, not an error
		{"0.00000001", 1},    // the smallest expressible rate
		{"0.000000001", 0},   // finer than Scale — truncates to nothing, refused
		{"1e5", 0},           // scientific notation is not a price this parses
		{"", 0},              // empty
		{"-5", 0},            // negative
		{"abc", 0},           // garbage
		{"0", 0},             // zero is not a price
	} {
		got, err := decimalToMicroCents(c.in)
		if c.want == 0 {
			if err == nil {
				t.Errorf("decimalToMicroCents(%q) = %d, want an error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("decimalToMicroCents(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("decimalToMicroCents(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestDecimalToMicroCentsNeverRoundsUp(t *testing.T) {
	// The property, not a table: digits finer than Scale are DROPPED, never
	// rounded up. A rail that rounded up would credit value nobody sent, on every
	// deposit, forever. These all carry 8 decimals already, so the extra digit is
	// past the scale and must change nothing.
	for _, s := range []string{"1.99999999", "0.01999999", "12345.99999999", "7.00500000"} {
		got, err := decimalToMicroCents(s)
		if err != nil {
			t.Fatalf("decimalToMicroCents(%q): %v", s, err)
		}
		up, err := decimalToMicroCents(s + "9")
		if err != nil {
			t.Fatalf("decimalToMicroCents(%q): %v", s+"9", err)
		}
		if up != got {
			t.Errorf("%q=%d but %q=%d — a digit past the scale changed the rate, so it is not truncating", s, got, s+"9", up)
		}
	}
}
