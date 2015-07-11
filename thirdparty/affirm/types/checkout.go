package types

type Checkout struct {
	Config         CheckoutConfig `json:"config"`          // The checkout config
	Merchant       MerchantConfig `json:"merchant"`        // The merchant config
	Billing        Contact        `json:"billing"`         // The billing contact
	Shipping       Contact        `json:"shipping"`        // The shipping contact
	Items          []Item         `json:"items"`           // A list of item objects
	TaxAmount      int            `json:"tax_amount"`      // The total tax amount computed after all discounts have been applied.  Defaults to 0.
	ShippingAmount int            `json:"shipping_amount"` // The total shipping amount, defaults to 0.
	Total          int            `json:"total"`           // The total amount of the checkout.  This determines the total amount charged to the user.
}
