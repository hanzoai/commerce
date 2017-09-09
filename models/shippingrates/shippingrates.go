package shippingrates

import (
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/georate"
)

type GeoRate struct {
	georate.GeoRate

	// Shipping Name like 'Shipping'
	ShippingName string `json:"shippingName`
}

type ShippingRates struct {
	mixin.Model

	StoreId string `json:"storeId"`

	GeoRates []GeoRate `json:"geoRates"`
	// TODO: Support Mass / Dimension Based Rates
	// DimRates []DimRate `json:"dimRates"`
}
