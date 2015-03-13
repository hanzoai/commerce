package models

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

type CurrencyType string

func (t CurrencyType) Symbol() string {
	switch t {
	case USD, AUD, CAD:
		return "$"
	case EUR:
		return "€"
	case GBP:
		return "£"
	}

	return ""
}

const (
	USD CurrencyType = "usd"
	AUD              = "aud"
	CAD              = "cad"
	EUR              = "eur"
	GBP              = "gbp"
)
