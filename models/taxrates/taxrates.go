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

func (t TaxRates) GetGeoRates() []georate.GeoRate {
	grs := make([]georate.GeoRate, 0)
	for i, _ := range t.GeoRates {
		grs = append(grs, t.GeoRates[i].GeoRate)
	}
	return grs
}

func (t TaxRates) Match(ctr, st, ct, pc string) (*GeoRate, int, int) {
	gr, level, i := georate.Match(t.GetGeoRates(), ctr, st, ct, pc)
	if gr != nil {
		return &t.GeoRates[i], level, i
	}

	return nil, level, i
}
