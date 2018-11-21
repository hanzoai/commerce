package variant

import (
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/productcachedvalues"
	"hanzo.io/util/val"

	. "hanzo.io/types"
)

type Variant struct {
	mixin.Model
	mixin.Salesforce
	productcachedvalues.ProductCachedValues

	ProductId string `json:"productId"`

	SKU string `json:"sku"`
	UPC string `json:"upc,omitempty"`

	Name string `json:"name"`

	// Variant Media
	Header Media   `json:"header"`
	Image  Media   `json:"image"`
	Media  []Media `json:"media"`

	// Is the variant available
	Available bool `json:"available"`

	// Range in which variant is available. If active, it takes precedent over
	// Available bool.
	Availability Availability `json:"availability"`

	// Inventory int `json:"inventory"`
	Sold int `json:"sold"`

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
