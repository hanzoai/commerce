package models

import "crowdstart.io/models/mixin"

type LineItem struct {
	mixin.Salesforce

	ProductId   string `json:"productId"`
	ProductName string `json:"productName"`
	ProductSlug string `json:"productSlug"`

	CollectionId string `json:"collectionId"`

	VariantId   string `json:"variantId"`
	VariantName string `json:"variantName"`
	VariantSKU  string `json:"variantSKU"`

	// Unit price
	Price Cents `json:"price"`

	// Number of units
	Quantity int `json:"quantity"`

	// Unit weight
	Weight int `json:"weight"`

	// Whether taxes apply to this line item
	Taxable bool `json:"taxable"`
}

func (li LineItem) TotalPrice() Cents {
	return li.Price * Cents(li.Quantity)
}

func (li LineItem) DisplayPrice() string {
	return DisplayPrice(li.TotalPrice())
}

// func (li LineItem) Validate(req *http.Request, errs binding.Errors) binding.Errors {
// 	if li.SKU() == "" {
// 		errs = append(errs, binding.Error{
// 			FieldNames:     []string{"Variant.SKU"},
// 			Classification: "InputError",
// 			Message:        "SKU cannot be empty.",
// 		})
// 	}

// 	if li.Quantity < 1 {
// 		errs = append(errs, binding.Error{
// 			FieldNames:     []string{"Quantity"},
// 			Classification: "InputError",
// 			Message:        "Quantity cannot be less than 1.",
// 		})
// 	}

// 	return errs
// }

// Displays nice "/" delimited variant information.
// func (li LineItem) DisplayShortDescription() string {
// 	opts := []string{}
// 	for _, opt := range []string{li.Product.Title, li.Variant.Color, li.Variant.Style, li.Variant.Size} {
// 		if opt != "" {
// 			opts = append(opts, opt)
// 		}
// 	}
// 	if len(opts) > 0 {
// 		return strings.Join(opts, " / ")
// 	} else {
// 		return li.SKU()
// 	}
// }
