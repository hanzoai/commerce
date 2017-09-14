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

func (t ShippingRates) GetGeoRates() []georate.GeoRate {
	grs := make([]georate.GeoRate, 0)
	for i, _ := range t.GeoRates {
		grs = append(grs, t.GeoRates[i].GeoRate)
	}
	return grs
}

func (t ShippingRates) Match(ctr, st, ct, pc string) (*GeoRate, int, int) {
	gr, level, i := georate.Match(t.GetGeoRates(), ctr, st, ct, pc)
	if gr != nil {
		return &t.GeoRates[i], level, i
	}

	return nil, level, i
}
