package taxrates

import (
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/georate"
)

type GeoRate struct {
	georate.GeoRate

	TaxShipping bool `json:"taxShipping`
	// Tax Name like 'Tax' or 'VAT'
	TaxName string `json:"taxName`
}

type TaxRates struct {
	mixin.Model

	StoreId string `json:"storeId"`

	GeoRates []GeoRate `json:"geoRates"`
}
