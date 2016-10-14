package variant

import (
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/types/weight"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
)

type Variant struct {
	mixin.Model
	mixin.Salesforce

	ProductId string `json:"productId"`

	SKU  string `json:"sku"`
	Name string `json:"name"`

	// 3-letter ISO currency code (lowercase).
	Currency currency.Type  `json:"currency"`
	Price    currency.Cents `json:"price"`

	// Variant Media
	Header Media   `json:"header"`
	Image  Media   `json:"image"`
	Media  []Media `json:"media"`

	// Is the variant available
	Available bool `json:"available"`

	// Range in which variant is available. If active, it takes precedent over
	// Available bool.
	Availability Availability `json:"availability"`

	Inventory int `json:"inventory"`
	Sold      int `json:"sold"`

	Weight     weight.Mass `json:"weight"`
	WeightUnit weight.Unit `json:"weightUnit"`
	Dimensions string      `json:"dimensions"`

	Taxable bool `json:"taxable"`

	Options []Option `json:"options"`
}

func (v *Variant) Validator() *val.Validator {
	return val.New().Check("ProductId").Exists().
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
