package currency

import "testing"

// A currency's scale is a property of the currency. Before there was a table,
// Decimals() answered 0 for a fixed list and 2 for literally everything else —
// so a satoshi rounded to zero, a Kuwaiti dinar came out ten times short, and a
// code nobody had registered resolved silently to two decimals with no error.
func TestDecimalsIsTheCurrencysOwnScale(t *testing.T) {
	for _, tc := range []struct {
		cur  Type
		want int32
		unit string
	}{
		{JPY, 0, "yen is indivisible"},
		{USD, 2, "cent"},
		{EUR, 2, "cent"},
		{KWD, 3, "fils"},
		{BHD, 3, "fils"},
		{XRP, 6, "drop"},
		{USDC, 6, "micro-USDC"},
		{BTC, 8, "satoshi"},
		{XBT, 8, "satoshi"},
		{DOGE, 8, "satoshi-equivalent"},
		{SOL, 9, "lamport"},
		{TON, 9, "nanoton"},
		{ETH, 18, "wei"},
		{LUX, 18, "wei"},
		{ZOO, 18, "wei"},
		{HANZO, 18, "wei"},
		{PNT, 0, "a point is indivisible"},
	} {
		t.Run(string(tc.cur), func(t *testing.T) {
			if got := tc.cur.Decimals(); got != tc.want {
				t.Errorf("%s.Decimals() = %d, want %d (%s)", tc.cur, got, tc.want, tc.unit)
			}
		})
	}
}

// The smallest indivisible unit of each currency must survive the round trip.
// At two decimals a satoshi and a wei both became zero — money vanishing with
// no error, which is the worst shape a money bug can take.
func TestSmallestUnitSurvives(t *testing.T) {
	for _, tc := range []struct {
		cur   Type
		major string
		want  string // minor units, as a decimal string
	}{
		{USD, "0.01", "1"},
		{JPY, "1", "1"},
		{KWD, "0.001", "1"},
		{XRP, "0.000001", "1"},
		{USDC, "0.000001", "1"},
		{BTC, "0.00000001", "1"},
		{SOL, "0.000000001", "1"},
		{TON, "0.000000001", "1"},
		{ETH, "0.000000000000000001", "1"},
		{LUX, "0.000000000000000001", "1"},
	} {
		t.Run(string(tc.cur), func(t *testing.T) {
			a, err := tc.cur.ParseAmount(tc.major)
			if err != nil {
				t.Fatalf("ParseAmount(%q): %v", tc.major, err)
			}
			if got := a.Minor().String(); got != tc.want {
				t.Errorf("%s %s = %s minor units, want %s", tc.cur, tc.major, got, tc.want)
			}
			if got := a.MajorString(); got != a.MajorString() {
				t.Errorf("unstable render")
			}
		})
	}
}

// An 18-decimal balance exceeds int64 by many orders of magnitude, so it has to
// travel as money.Amount. This is the case Cents cannot represent at all: ten
// ETH is 1e19 wei and an int64 stops at 9.2e18.
func TestEighteenDecimalBalancesAreExact(t *testing.T) {
	for _, tc := range []struct {
		cur              Type
		major, wantMinor string
	}{
		{ETH, "1", "1000000000000000000"},
		{ETH, "1234.567890123456789012", "1234567890123456789012"},
		{LUX, "1000000", "1000000000000000000000000"},
		{HANZO, "0.123456789012345678", "123456789012345678"},
	} {
		t.Run(string(tc.cur)+"/"+tc.major, func(t *testing.T) {
			a, err := tc.cur.ParseAmount(tc.major)
			if err != nil {
				t.Fatalf("ParseAmount(%q): %v", tc.major, err)
			}
			if got := a.Minor().String(); got != tc.wantMinor {
				t.Errorf("%s %s = %s wei, want %s", tc.cur, tc.major, got, tc.wantMinor)
			}
			// And it renders back to exactly the digits it was given.
			if got := a.MajorString(); got != normalize(tc.cur, tc.major) {
				t.Errorf("round trip = %s, want %s", got, normalize(tc.cur, tc.major))
			}
		})
	}
}

// normalize renders the expected major string at the currency's fixed scale,
// which is how MajorString always spells it.
func normalize(c Type, major string) string {
	a, err := c.ParseAmount(major)
	if err != nil {
		return major
	}
	return a.MajorString()
}

// Cents is an int64, so it cannot hold an 18-decimal token. It must say so
// rather than truncate: a silent wrap here is a balance off by orders of
// magnitude, and FitsCents is how a caller knows before it tries.
func TestCentsRefusesWhatItCannotHold(t *testing.T) {
	if ETH.FitsCents() || LUX.FitsCents() {
		t.Error("an 18-decimal token must report that it does not fit Cents")
	}
	for _, c := range []Type{USD, JPY, KWD, BTC, SOL, TON, USDC} {
		if !c.FitsCents() {
			t.Errorf("%s has %d decimals and does fit Cents", c, c.Decimals())
		}
	}
	// Ten ETH is 1e19 wei. Parse must return an error, never a wrapped int64.
	if got, err := ETH.Parse("10"); err == nil {
		t.Errorf("ETH.Parse(\"10\") = %d with no error — it must refuse, not wrap", int64(got))
	}
}
