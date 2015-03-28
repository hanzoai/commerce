package variant

import (
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/types/currency"
	"crowdstart.io/util/val"

	. "crowdstart.io/models2"
)

type Option struct {
	// Ex. Size
	Name string `json:"name"`
	// Ex. M
	Value string `json:"value"`
}

type Variant struct {
	mixin.Model
	mixin.Salesforce

	ProductId string `json:"productId"`

	SKU  string `json:"sku"`
	Name string `json:"name"`

	// 3-letter ISO currency code (lowercase).
	Currency currency.Type  `json:"currency"`
	Price    currency.Cents `json:"price"`
	Discount currency.Cents `json:"discount"`

	// Is the variant available
	Available bool `json:"available"`

	// Range in which variant is available. If active, it takes precedent over
	// Available bool.
	Availability struct {
		Active    bool
		StartDate time.Time `json:"startDate"`
		EndDate   time.Time `json:"endDate"`
	} `json:"availability"`

	Inventory int `json:"inventory"`
	Sold      int `json:"sold"`

	Weight     float64  `json:"weight"`
	WeightUnit MassUnit `json:"weightUnit"`
	Dimensions string   `json:"dimensions"`

	Options []Option `json:"options"`
}

func New(db *datastore.Datastore) *Variant {
	v := new(Variant)
	v.Options = make([]Option, 0)
	v.Model = mixin.Model{Db: db, Entity: v}
	return v
}

func (v Variant) Kind() string {
	return "variant2"
}

func (v *Variant) Validator() *val.Validator {
	return val.New(v).Check("ProductId").Exists().
		Check("SKU").Exists().
		Check("Name").Exists()

	// if v.SKU == "" {
	// 	errs = append(errs, binding.Error{
	// 		FieldNames:     []string{"SKU"},
	// 		Classification: "InputError",
	// 		Message:        "Variant does not have a SKU",
	// 	})
	// }

	// if v.Dimensions == "" {
	// 	errs = append(errs, binding.Error{
	// 		FieldNames:     []string{"Dimensions"},
	// 		Classification: "InputError",
	// 		Message:        "Variant has no given dimensions",
	// 	})
	// }
	// return errs
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
