package currency

import (
	"math/big"
	"strings"

	"github.com/hanzoai/money"
)

type Type string

// decimals is the table of minor-unit scale for a currency SYMBOL.
//
// A symbol is not always enough. Decimals are a property of (chain, asset), not
// of the ticker: USDC is 6 on Ethereum, Base and Solana but 18 bridged onto Lux
// C-Chain, and LUX itself is 18 on the C-Chain and 9 on the X- and P-Chains.
// The scales below are the denomination an ORDER is priced in, which is the
// mainnet/EVM one. Anything reading a bridged balance off a specific chain must
// take the scale from that chain's asset record, not from here.
//
// Original note follows.
//
// This is the ONE table of minor-unit scale. A currency's minor unit is
// whatever that currency actually cannot subdivide — a fils, a satoshi, a wei —
// and it is a property of the currency, never of the code reading it.
//
// Anything absent is two decimals, which is the ISO default for fiat. That
// default is why this table has to be explicit: a code nobody registered used
// to resolve silently to 2, so a satoshi rounded to zero and a Kuwaiti dinar
// came out ten times short, with no error either time.
var decimals = map[Type]int32{
	// Zero-decimal fiat (ISO 4217). The minor unit IS the major unit.
	BIF: 0, CLP: 0, DJF: 0, GNF: 0, JPY: 0, KMF: 0, KRW: 0, MGA: 0,
	PYG: 0, RWF: 0, VND: 0, VUV: 0, XAF: 0, XOF: 0, XPF: 0,

	// Three-decimal fiat (ISO 4217). One dinar is 1000 fils.
	BHD: 3, IQD: 3, JOD: 3, KWD: 3, LYD: 3, OMR: 3, TND: 3,

	// Crypto, at the chain's own smallest indivisible unit.
	XRP: 6, USDC: 6, USDT: 6,
	BTC: 8, XBT: 8, BCH: 8, LTC: 8, DOGE: 8,
	SOL: 9, TON: 9,
	ETH: 18, MATIC: 18, AVAX: 18, SHIB: 18,

	// Own chains — EVM-native, so wei like any other EVM gas token.
	// LUX here is the C-Chain (EVM) denomination, which is what an order is
	// priced in. LUX on the X- and P-Chains is 9, and a caller that means those
	// has to say so; a bare symbol cannot carry which chain it came from.
	LUX: 18, ZOO: 18, AI: 18, HANZO: 18, SPC: 18, PARS: 18, HUSD: 18,

	// A point is indivisible.
	PNT: 0,
}

// Does the currency not have a decimal convention such as Japanese Yen (¥100) instead
// of USD ($1.00)
func (t Type) IsZeroDecimal() bool { return t.Decimals() == 0 }

// Decimals is how many fractional digits the currency's minor unit has. This is
// the ONE place that scale is decided; nothing else may hardcode a 100.
func (t Type) Decimals() int32 {
	if d, ok := decimals[t]; ok {
		return d
	}
	return 2
}

// FitsCents reports whether this currency's minor units fit the int64 that
// Cents is. They do not for an 18-decimal token: one ETH is 1e18 wei and an
// int64 tops out at 9.2e18, so Cents cannot hold ten of them. Those amounts
// have to travel as money.Amount, which is arbitrary-precision.
func (t Type) FitsCents() bool { return t.Decimals() <= 9 }

// Money lifts this currency into github.com/hanzoai/money — the ONE bridge
// between commerce's currency table and the package that owns exact money.
func (t Type) Money() money.Currency {
	return money.Currency{Code: t.Code(), Decimals: t.Decimals(), Symbol: t.Symbol()}
}

// ParseAmount reads a decimal string of MAJOR units into an exact amount at this
// currency's own scale, with no int64 anywhere on the path.
//
// This is the conversion to reach for when the currency might be an 18-decimal
// token: one ETH is 1e18 wei, so Parse (which returns Cents, an int64) can only
// carry nine of them before it overflows. money.Amount is arbitrary-precision —
// a big.Int underneath — so it holds a wei and a whale balance alike.
func (t Type) ParseAmount(s string) (money.Amount, error) {
	return money.Parse(s, t.Money())
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

// Parse is the inverse of ToStringNoSymbol: it reads a decimal string in this
// currency's major unit and returns exact minor units.
//
// It exists so an amount that arrives as a DECIMAL never has to pass through a
// float64 to become Cents. "19.99" has no exact binary representation, so
// Cents(f*100) on a parsed float yields 1998 — a cent lost, on money that was
// already captured. The scale comes from this package's own currency table
// rather than money's registry, which knows 29 of the 142 currencies here.
func (t Type) Parse(s string) (Cents, error) {
	minor, err := money.ParseMinor(s, t.Money())
	if err != nil {
		return 0, err
	}
	return Cents(minor), nil
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
