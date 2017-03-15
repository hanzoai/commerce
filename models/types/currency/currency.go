package currency

import (
	"strconv"
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

func (t Type) ToString(c Cents) string {
	if t.IsZeroDecimal() {
		return t.Symbol() + strconv.Itoa(int(c))
	}
	cents := strconv.Itoa(int(c) % 100)
	if len(cents) < 2 {
		cents += "0"
	}
	return t.Symbol() + strconv.Itoa(int(c)/100) + "." + cents
}

func (t Type) ToStringNoSymbol(c Cents) string {
	if t.IsZeroDecimal() {
		return strconv.Itoa(int(c))
	}
	cents := strconv.Itoa(int(c) % 100)
	if len(cents) < 2 {
		cents = "0" + cents
	}
	return strconv.Itoa(int(c)/100) + "." + cents
}

func (t Type) ToFloat(c Cents) float64 {
	if t.IsZeroDecimal() {
		return float64(c)
	}
	return float64(c) / 100.0
}

func (t Type) Label() string {
	return t.Symbol() + " " + t.Code()
}

func (t Type) Code() string {
	return strings.ToUpper(string(t))
}
