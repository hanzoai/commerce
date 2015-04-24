package currency

import (
	"fmt"
	"strings"

	"crowdstart.io/util/log"
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

func (c Cents) Humanize() string {
	t := USD
	cents := c % 100
	dollars := c / 100
	log.Warn("%s%d.%d", t, dollars, cents)
	return fmt.Sprintf("%s%d.%d", t.Symbol(), dollars, cents)
}

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
