package xrplrpc

import (
	"context"
	"math/big"
	"strings"
	"testing"
)

// THE test this package exists for.
//
// An XRPL issued amount is a decimal STRING and not integer base units, so the
// step every other chain gets from the chain itself — "how many base units is
// this?" — is done here, by us, in code. A parser that is off by one power of
// ten credits ten times the money. The cases below are therefore stated as
// exact integers rather than computed, so an error in the parser cannot be
// mirrored by the same error in the expectation.
func TestParseValue(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string // base units at Scale (15), exact decimal
	}{
		// The plain cases, at the scale a dollar-pegged token is actually used.
		{"1", "1000000000000000"},
		{"1234.56", "1234560000000000000"},
		{"0.003", "3000000000000"},
		{"0.01", "10000000000000"},
		{"845703146.2015528", "845703146201552800000000"},
		{"0", "0"},
		{"0.0", "0"},
		{"00012.5", "12500000000000000"},
		{".5", "500000000000000"},
		{"5.", "5000000000000000"},

		// Exponent forms. rippled renders very large and very small values this
		// way, so a parser that only understood plain decimals would either
		// refuse a real payment or — far worse — misread one.
		{"1e-5", "10000000000"},
		{"1E-5", "10000000000"},
		{"1.5e3", "1500000000000000000"},
		{"1e+3", "1000000000000000000"},
		{"15e-1", "1500000000000000"},

		// Truncation is toward zero, ALWAYS, so a credit can never exceed what
		// was sent. Below the rendering scale a value becomes zero, which the
		// watcher reports as dust.
		{"0.0000000000000019", "1"},
		{"0.0000000000000001", "0"},
		{"1e-16", "0"},
		{"1e-90", "0"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseValue(tc.in)
			if err != nil {
				t.Fatalf("ParseValue(%q): %v", tc.in, err)
			}
			if got.String() != tc.want {
				t.Fatalf("ParseValue(%q) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

// The safety property that REPLACES reading decimals off the chain.
//
// On the EVM the scale is trustworthy because the contract states it. Here
// there is nothing to state it, so the guarantee has to be different in kind:
// the function that parses the ledger's number and the number Decimals()
// reports must describe the same thing. This asserts exactly that — units
// divided by 10^Decimals is the value the ledger stated — which is what makes
// the cent arithmetic downstream correct.
func TestParseValue_RoundTripsAtDecimals(t *testing.T) {
	c := NewClient("http://unused", Issued{})
	decimals, err := c.Decimals(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if decimals != Scale {
		t.Fatalf("Decimals() = %d but values are parsed at %d — the scale and the parse disagree", decimals, Scale)
	}
	scale := new(big.Rat).SetInt(pow10(decimals))

	// big.Rat, NOT big.Float: 1234.56 has no exact binary representation, so a
	// float comparison would fail on a correct parser and — much worse — could
	// pass on a wrong one. The whole point of this test is exactness.
	for _, in := range []string{"1", "1234.56", "0.003", "9999999.999999", "1e-5", "845703146.2015528", "0.0000000000000019"} {
		units, err := ParseValue(in)
		if err != nil {
			t.Fatalf("ParseValue(%q): %v", in, err)
		}
		got := new(big.Rat).Quo(new(big.Rat).SetInt(units), scale)
		want, ok := new(big.Rat).SetString(in)
		if !ok {
			t.Fatalf("parse expectation %q", in)
		}
		// Truncation toward zero is by design, so the parsed value may be
		// SMALLER than the stated one by less than one unit at Scale — never
		// larger, which is the direction that would credit money not sent.
		if got.Cmp(want) > 0 {
			t.Fatalf("%q parsed to %s units = %s, which is MORE than the ledger stated", in, units, got.RatString())
		}
		slack := new(big.Rat).Sub(want, got)
		if slack.Cmp(new(big.Rat).SetFrac(big.NewInt(1), pow10(decimals))) >= 0 {
			t.Fatalf("%q parsed to %s units = %s, which is %s short of %s — more than one unit at %d decimals",
				in, units, got.RatString(), slack.RatString(), want.RatString(), decimals)
		}
	}
}

// Everything outside the grammar is REFUSED rather than coerced. rippled
// renders these values itself, so an input this parser does not understand is a
// response we do not understand — and crediting money from one of those is the
// failure the whole rail exists to end.
func TestParseValue_RefusesWhatItCannotJustify(t *testing.T) {
	for _, tc := range []struct{ name, in string }{
		{"empty", ""},
		{"blank", "   "},
		{"negative", "-1"},
		{"negative zero", "-0"},
		{"not a number", "abc"},
		{"two decimal points", "1.2.3"},
		{"a plus sign", "+1"},
		{"hex", "0x10"},
		{"thousands separator", "1,000"},
		{"exponent with no digits", "1e"},
		{"exponent that is not a number", "1eX"},
		{"no mantissa", "e5"},
		{"an exponent past the ledger's own range", "1e200"},
		// The memory bomb: 10^n for an attacker-supplied n. It must be refused
		// by inspection, never by attempting the allocation.
		{"an absurd exponent", "1e999999999999999999999"},
		{"an absurd negative exponent", "1e-999999999999999999999"},
		{"whitespace inside", "1 000"},
		{"unavailable", "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseValue(tc.in)
			if err == nil {
				t.Fatalf("ParseValue(%q) = %s, want a refusal", tc.in, got)
			}
		})
	}
}

// A 3-character code and its 40-hex spelling are ONE currency on this ledger,
// and the ledger renders whichever it feels like. Comparing the rendered
// strings would make them two tokens — one of which nothing would ever match.
func TestParseCurrency(t *testing.T) {
	usd, err := ParseCurrency("USD")
	if err != nil {
		t.Fatal(err)
	}
	usdHex, err := ParseCurrency("0000000000000000000000005553440000000000")
	if err != nil {
		t.Fatal(err)
	}
	if usd != usdHex {
		t.Fatalf("USD (%s) and its hex spelling (%s) are not the same currency", usd, usdHex)
	}
	if got := usd.Symbol(); got != "USD" {
		t.Fatalf("USD symbol = %q", got)
	}

	// A non-standard code: the real RLUSD, ASCII left-aligned and NUL padded.
	rl, err := ParseCurrency("524C555344000000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if got := rl.Symbol(); got != "RLUSD" {
		t.Fatalf("RLUSD symbol = %q, want RLUSD", got)
	}
	if rl == usd {
		t.Fatal("RLUSD and USD compare equal")
	}

	// A ticker of four or more characters is the SAME 160 bits as its hex
	// spelling. Configuration is written the readable way; the ledger renders
	// the other; they must be one token.
	usdc, err := ParseCurrency("USDC")
	if err != nil {
		t.Fatal(err)
	}
	usdcHex, err := ParseCurrency("5553444300000000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if usdc != usdcHex {
		t.Fatalf("USDC (%s) and its hex spelling (%s) are not the same currency", usdc, usdcHex)
	}
	if got := usdc.Symbol(); got != "USDC" {
		t.Fatalf("USDC symbol = %q", got)
	}
	// ...and a 4-character ticker is NOT the standard 3-character layout, so it
	// must not collide with a 3-character code.
	if usdc == usd {
		t.Fatal("USDC and USD compare equal")
	}

	// Case is SIGNIFICANT: the ledger compares 160-bit values, so "usd" is a
	// different token from "USD" and folding them would merge two issuers'
	// books.
	lower, err := ParseCurrency("usd")
	if err != nil {
		t.Fatal(err)
	}
	if lower == usd {
		t.Fatal("\"usd\" and \"USD\" compare equal — currency codes are case-significant")
	}
}

func TestParseCurrency_Refusals(t *testing.T) {
	for _, tc := range []struct{ name, in string }{
		{"XRP as a 3-char code", "XRP"},
		{"XRP as the all-zero code", "0000000000000000000000000000000000000000"},
		{"XRP left-aligned in hex", "5852500000000000000000000000000000000000"},
		{"longer than the 20 bytes a code has", "AVERYLONGTICKERINDEEDXX"},
		{"not printable ASCII", "US₮D"},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := ParseCurrency(tc.in); err == nil {
				t.Fatalf("ParseCurrency(%q) = %s, want a refusal", tc.in, got)
			}
		})
	}
}

// An r-address carries a checksum, so a typo does not survive parsing. That is
// the difference between a custody address failing at boot and failing at the
// first deposit.
func TestParseAccount(t *testing.T) {
	const rlusdIssuer = "rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De"
	if _, err := ParseAccount(rlusdIssuer); err != nil {
		t.Fatalf("a real mainnet issuer was refused: %v", err)
	}
	if _, err := ParseAccount(" " + rlusdIssuer + " "); err != nil {
		t.Fatalf("surrounding whitespace was not trimmed: %v", err)
	}
	for _, tc := range []struct{ name, in string }{
		{"empty", ""},
		{"one character changed", "rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5Df"},
		{"two characters transposed", "rMxCKbEDwqr76QuheSUMdEGf4B9xJ85mDe"},
		{"truncated", "rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5D"},
		{"not base58", "rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5D0"},
		{"an EVM address", "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"},
		// An X-address encodes a destination tag INSIDE the address. This rail
		// carries the tag in its own field, so accepting one would silently
		// route a deposit to whatever tag was hidden in the string.
		{"an X-address", "X7AcgcsBL6XDcUb289X4mJ8djcdyKaB5hJDWMArnXr61cqZ"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseAccount(tc.in); err == nil {
				t.Fatalf("ParseAccount(%q) was accepted", tc.in)
			}
		})
	}
}

// The configured token is the PAIR. Neither half names a token alone, and a
// slot that took only one of them would let a deposit from any issuer of a
// same-named currency be credited at a dollar.
func TestParseIssued(t *testing.T) {
	is, err := ParseIssued("RLUSD.rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De")
	if err != nil {
		t.Fatal(err)
	}
	if is.Currency.Symbol() != "RLUSD" || is.Issuer != "rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De" {
		t.Fatalf("parsed to %+v", is)
	}
	for _, tc := range []struct{ name, in string }{
		{"no issuer", "RLUSD"},
		{"no currency", ".rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De"},
		{"trailing dot", "RLUSD."},
		{"a bad issuer checksum", "RLUSD.rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5Df"},
		{"XRP", "XRP.rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De"},
		{"a TON address", "EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := ParseIssued(tc.in); err == nil {
				t.Fatalf("ParseIssued(%q) = %s, want a refusal", tc.in, got)
			}
		})
	}
}

// A destination tag of 0 is a LEGAL tag somebody may hold. "No tag at all" is a
// different fact, and collapsing the two would credit an untagged stranger's
// payment to whichever customer was issued tag 0.
func TestDestinationTag_ZeroIsNotAbsent(t *testing.T) {
	zero := uint32(0)
	if got := destinationTag(&zero); got != "0" {
		t.Fatalf("tag 0 rendered as %q, want \"0\"", got)
	}
	if got := destinationTag(nil); got != "" {
		t.Fatalf("an absent tag rendered as %q, want the empty string", got)
	}
	if destinationTag(&zero) == destinationTag(nil) {
		t.Fatal("tag 0 and no tag are indistinguishable")
	}
}

// delivered_amount, and never Amount. With tfPartialPayment set, Amount is the
// most the sender was willing to deliver and the ledger may deliver a fraction
// of it — the partial-payment exploit that has drained exchanges. This asserts
// the client reads the ledger's own statement of what arrived.
func TestDelivered_ReadsDeliveredAmountAndChecksBothHalvesOfTheToken(t *testing.T) {
	token, err := ParseIssued("RLUSD.rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De")
	if err != nil {
		t.Fatal(err)
	}
	c := NewClient("http://unused", token)
	const ours = `{"currency":"524C555344000000000000000000000000000000","issuer":"rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De","value":"12.5"}`

	units, ok, err := c.delivered(&txMeta{DeliveredAmount: []byte(ours)}, "HASH")
	if err != nil || !ok {
		t.Fatalf("our own token was not recognised: ok=%v err=%v", ok, err)
	}
	if units.String() != "12500000000000000" {
		t.Fatalf("delivered 12.5 read as %s units", units)
	}

	for _, tc := range []struct {
		name string
		body string
	}{
		{
			// The attack this check exists for: same ticker, different issuer.
			name: "our currency from an issuer we did not configure",
			body: `{"currency":"524C555344000000000000000000000000000000","issuer":"rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B","value":"1000000"}`,
		},
		{
			name: "our issuer, a different currency",
			body: `{"currency":"USD","issuer":"rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De","value":"1000000"}`,
		},
		{
			name: "native XRP, which this rail cannot price",
			body: `"5000000"`,
		},
		{
			name: "nothing delivered",
			body: `null`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ok, err := c.delivered(&txMeta{DeliveredAmount: []byte(tc.body)}, "HASH")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok {
				t.Fatal("credited a delivery that is not our token")
			}
		})
	}

	// "unavailable" means the metadata cannot say how much arrived. That must
	// stop the pass, never quietly credit nothing.
	if _, _, err := c.delivered(&txMeta{DeliveredAmount: []byte(`"unavailable"`)}, "HASH"); err == nil {
		t.Fatal("an unreadable delivered amount was treated as no delivery")
	} else if !strings.Contains(err.Error(), "delivered amount") {
		t.Fatalf("error %q does not explain the problem", err)
	}
}
