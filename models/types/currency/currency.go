package currency

import (
	"math/big"
	"strings"

	"github.com/hanzoai/money"
)

type Type string

// Does the currency not have a decimal convention such as Japanese Yen (¥100) instead
// of USD ($1.00)
func (t Type) IsZeroDecimal() bool {
	switch t {
	case BIF, CLP, DJF, GNF, JPY, KMF, KRW, MGA, PYG, RWF, VND, VUV, XAF, XOF, XPF:
		return true
	}

	return false
}

// Decimals is how many fractional digits the currency's minor unit has — 0 for
// a zero-decimal currency such as JPY, 2 for everything else. This is the ONE
// place that scale is decided; nothing else may hardcode a 100.
func (t Type) Decimals() int32 {
	if t.IsZeroDecimal() {
		return 0
	}
	return 2
}

// Money lifts this currency into github.com/hanzoai/money — the ONE bridge
// between commerce's currency table and the package that owns exact money.
func (t Type) Money() money.Currency {
	return money.Currency{Code: t.Code(), Decimals: t.Decimals(), Symbol: t.Symbol()}
}

// Amount lifts a minor-unit amount in this currency into an exact money.Amount.
//
// This is the ONE conversion out of Cents, and every rendering hangs off it:
// .MajorString() for a decimal string on the wire, .Display() for a human,
// .AsMajorUnits() for the last inch of a third-party API whose schema demands a
// JSON number. commerce owns no arithmetic of its own here — money does, on a
// big.Int, so no rendering can lose a cent or invent one.
func (t Type) Amount(c Cents) money.Amount {
	return money.FromMinor(int64(c), t.Money())
}

// ToString renders the amount with its symbol ("$10.99", "-$19.99").
func (t Type) ToString(c Cents) string {
	s := t.ToStringNoSymbol(c)
	if strings.HasPrefix(s, "-") {
		return "-" + t.Symbol() + s[1:]
	}
	return t.Symbol() + s
}

// ToStringNoSymbol renders the amount as a plain fixed-scale decimal string
// ("10.99", "500" for a zero-decimal currency, "-19.99" for a refund).
func (t Type) ToStringNoSymbol(c Cents) string {
	return t.Amount(c).MajorString()
}

// Give the currency's Symbol + Code string
func (t Type) Label() string {
	return t.Symbol() + " " + t.Code()
}

// Give the currency's Code
func (t Type) Code() string {
	return strings.ToUpper(string(t))
}

// ------ More or Less Crypto Specific ------

// Is this a supported cryptocurrency
func (t Type) IsCrypto() bool {
	switch t {
	case BTC, ETH, XBT:
		return true
	}

	return false
}

// Since pricing things in a crypto minimal denomination exceed int64 and the
// minimal domination is worth so little, we generally use a larger
// denomination of the currency by convention that can capture the minimal
// relatable values.
//
// This returns the ratio of convention denomination to minimal denomination
func (t Type) MinimalUnitFactor() *big.Int {
	switch t {
	case ETH:
		//ETH is priced in Gwei or 1e-9 ETH or 0.000000001 ETH
		//Gwei or 1e9 ETH or 1,000,000,000 Wei so convert to wei
		return big.NewInt(1e9)
	}

	return big.NewInt(1)
}

func (t Type) ToMinimalUnits(c Cents) *big.Int {
	b := big.NewInt(int64(c))
	return b.Mul(b, t.MinimalUnitFactor())
}

func (t Type) FromMinimalUnits(b *big.Int) Cents {
	c := big.NewInt(0).Set(b)
	c = c.Div(c, t.MinimalUnitFactor())
	return Cents(c.Int64())
}
