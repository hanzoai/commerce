package georate

import (
	"hanzo.io/models/types/currency"
)

type GeoRate struct {
	Country string `json:"country"`
	State   string `json:"state"`

	// Only take a city OR postal code, not both
	City string `json:"city"`
	// Comma separates postal codes
	PostalCodes string `json:"postalCode"`

	// TODO: Support Product Category Tags
	// ProductCategory string `json:"productCategory"`

	// Support both percent and currency
	// Use store's currency in implementation
	Percent float64        `json:"percent"`
	Cost    currency.Cents `json:"cost"`
}
