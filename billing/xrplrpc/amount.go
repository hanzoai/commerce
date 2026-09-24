package xrplrpc

import (
	"fmt"
	"math/big"
	"strings"
)

// Scale is the number of decimal places this client renders an XRPL issued
// amount at, and the number it reports from Decimals().
//
// READ THIS BEFORE CHANGING ANYTHING IN THIS FILE.
//
// On every other chain in this rail, decimals are READ from the chain — an
// ERC-20 answers decimals(), an SPL mint carries its own — and that read is
// what makes a mis-scaled credit unrepresentable rather than merely unlikely.
// XRPL has no such thing to read, and pretending otherwise would be the lie
// worth catching here: an issued currency has NO BASE UNIT. The ledger states
// an amount as a decimal NUMBER ("1234.56", "0.003", "1e-5") carrying up to 15
// significant digits with a decimal exponent, and there is no on-ledger field
// saying how to scale it, because nothing on the ledger needs one.
//
// So the scale is not read, and it is not configured either. It is fixed here,
// and the safety property is different in kind but not weaker:
//
//	the SAME function that parses the ledger's number reports the scale it
//	parsed it at, so the two cannot disagree.
//
// There is no second source — no constant in a config map, no decimals() answer
// from a different call — that could drift from the parse. What Decimals()
// returns is true BY CONSTRUCTION of ParseValue, and TestParseValue_RoundTrip
// pins exactly that: units / 10^Scale is the number the ledger stated.
//
// 15 is chosen because it is XRPL's own significant-digit limit for an issued
// amount, so a value the ledger can state at all is a value this scale can hold
// without inventing precision. Amounts smaller than 10^-15 truncate to zero and
// become depositwatch.ErrDust — which they are: 10^-15 of a dollar-pegged token
// is 10^-13 of a cent.
const Scale = 15

// maxExponent bounds the decimal exponent this parser will act on.
//
// XRPL's own issued-amount range is roughly 10^-96 … 10^80, so nothing the
// ledger can state comes near this bound. It exists because the exponent
// controls an allocation: 10^n for an attacker-supplied n is how a JSON field
// becomes an out-of-memory, and a money parser must not be the place that
// happens. Anything past it is refused rather than approximated.
const maxExponent = 128

// ParseValue converts an XRPL issued-currency amount — the decimal string in an
// Amount object's `value` field — into base units at Scale.
//
// It is deliberately strict about the grammar it accepts. rippled renders these
// values itself, so anything outside the grammar is not a number we failed to
// handle; it is a response we do not understand, and crediting money from a
// response we do not understand is the failure this whole rail exists to end.
//
// Truncation is toward zero, ALWAYS. A remainder below Scale is dropped rather
// than rounded, so this can never scale a value UP — the direction that would
// credit money that was not sent.
func ParseValue(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("xrplrpc: empty amount")
	}
	if s[0] == '-' {
		// A delivered amount is what an account RECEIVED. A negative one means
		// we are reading a field that is not what we think it is, so it is an
		// error and never a zero credit.
		return nil, fmt.Errorf("xrplrpc: amount %q is negative", s)
	}

	mant := s
	exp := 0
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mant = s[:i]
		var err error
		exp, err = parseExponent(s[i+1:])
		if err != nil {
			return nil, fmt.Errorf("xrplrpc: amount %q: %w", s, err)
		}
	}

	intPart, fracPart := mant, ""
	if i := strings.IndexByte(mant, '.'); i >= 0 {
		intPart, fracPart = mant[:i], mant[i+1:]
	}
	digits := intPart + fracPart
	if digits == "" || !allDigits(digits) {
		return nil, fmt.Errorf("xrplrpc: amount %q is not a decimal number", s)
	}
	n, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return nil, fmt.Errorf("xrplrpc: amount %q is not a decimal number", s)
	}

	// The value is n × 10^(exp − len(frac)); rendering it at Scale shifts it by
	// a further +Scale.
	shift := exp - len(fracPart) + Scale
	switch {
	case n.Sign() == 0:
		return big.NewInt(0), nil
	case shift > maxExponent:
		return nil, fmt.Errorf("xrplrpc: amount %q is too large to render at %d decimal places", s, Scale)
	case shift >= 0:
		return n.Mul(n, pow10(shift)), nil
	case -shift > len(digits):
		// Smaller than one unit at Scale even before dividing: the quotient is
		// zero, and computing it would be a pointless big.Int allocation.
		return big.NewInt(0), nil
	default:
		return n.Quo(n, pow10(-shift)), nil
	}
}

// parseExponent reads the exponent of a rendered value, bounded before it is
// ever used as an exponent.
func parseExponent(s string) (int, error) {
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	if s == "" || !allDigits(s) {
		return 0, fmt.Errorf("exponent %q is not an integer", s)
	}
	// Bound the LENGTH before the value, so "1e999999999999999999999" is
	// refused by inspection rather than by overflowing an int.
	if len(strings.TrimLeft(s, "0")) > 4 {
		return 0, fmt.Errorf("exponent %q is out of range", s)
	}
	n := 0
	for _, ch := range []byte(s) {
		n = n*10 + int(ch-'0')
	}
	if n > maxExponent+Scale {
		return 0, fmt.Errorf("exponent %q is out of range", s)
	}
	if neg {
		n = -n
	}
	return n, nil
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func pow10(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}
