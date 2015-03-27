package currency

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
	USD Type = "usd"
	AUD      = "aud"
	CAD      = "cad"
	EUR      = "eur"
	GBP      = "gbp"
)

var List = []Type{USD, AUD, CAD, EUR, GBP}
