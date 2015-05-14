package currency

import (
	"strconv"
	"strings"
)

// import (
// 	"github.com/mholt/binding"
// 	"net/http"
// )
// type Currency struct {
// 	value int64
// 	FieldMapMixin
// }

// func (c Currency) Validate(req *http.Request, errs binding.Errors) binding.Errors {
// 	return errs
// }

// func (c Currency) Add()    {}
// func (c Currency) Sub()    {}
// func (c Currency) Mul()    {}
// func (c Currency) String() {}

type Cents int

type Type string

func (t Type) Symbol() string {
	switch t {
	case USD, AUD, CAD, HKD, NZD:
		return "$"
	case EUR:
		return "€"
	case GBP:
		return "£"
	case JPY:
		return "¥"
	}

	return ""
}

func (t Type) IsZeroDecimal() bool {
	switch t {
	case JPY:
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

func (t Type) Label() string {
	return t.Symbol() + " " + strings.ToUpper(string(t))
}

const (
	USD Type = "usd"
	AUD      = "aud"
	CAD      = "cad"
	EUR      = "eur"
	GBP      = "gbp"
	HKD      = "hkd"
	JPY      = "jpy"
	NZD      = "nzd"
)

var Types = []Type{USD, AUD, CAD, EUR, GBP, HKD, JPY, NZD}
