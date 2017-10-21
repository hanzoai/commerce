package currency

import (
	"strings"
)

type Type string

func (t Type) IsZeroDecimal() bool {
	switch t {
	case BIF, CLP, DJF, GNF, JPY, KMF, KRW, MGA, PYG, RWF, VND, VUV, XAF, XOF, XPF:
		return true
	}

	return false
}

func (t Type) IsCrypto() bool {
	switch t {
	case BTC, ETH, XBT:
		return true
	}

	return false
}

func (t Type) ToString(c Cents) string {
	if t.IsZeroDecimal() {
		return t.Symbol() + c.String()
	}
	cents := c.Mod(NewInt(100)).String()
	if len(cents) < 2 {
		cents += "0"
	}
	return t.Symbol() + c.Div(NewInt(100)).String() + "." + cents
}

func (t Type) ToStringNoSymbol(c Cents) string {
	if t.IsZeroDecimal() {
		return c.String()
	}
	cents := c.Mod(NewInt(100)).String()
	if len(cents) < 2 {
		cents = "0" + cents
	}
	return c.Mod(NewInt(100)).String() + "." + cents
}

func (t Type) ToFloat(c Cents) float64 {
	f := c.Float64()
	if t.IsZeroDecimal() {
		return f
	}
	return f / 100.0
}

func (t Type) Label() string {
	return t.Symbol() + " " + t.Code()
}

func (t Type) Code() string {
	return strings.ToUpper(string(t))
}
