package variant

import (
	"net/http"

	"github.com/mholt/binding"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"

	. "crowdstart.io/models2"
)

type Option struct {
	// Ex. Size
	Name string
	// Ex. M
	Value string
}

type Variant struct {
	*mixin.Model `datastore:"-"`
	mixin.Salesforce

	SKU  string
	Name string

	Price Cents

	Inventory int
	Sold      int

	Weight     float64
	WeightUnit MassUnit
	Dimensions string

	Options []Option
}

func New(db *datastore.Datastore) *Variant {
	v := new(Variant)
	v.Model = mixin.NewModel(db, v)
	return v
}

func (v Variant) Kind() string {
	return "variant"
}

func (v Variant) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if v.SKU == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"SKU"},
			Classification: "InputError",
			Message:        "Variant does not have a SKU",
		})
	}

	if v.Dimensions == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Dimensions"},
			Classification: "InputError",
			Message:        "Variant has no given dimensions",
		})
	}
	return errs
}
