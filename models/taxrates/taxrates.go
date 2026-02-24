package taxrates

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/types/georate"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[TaxRates]("taxrates") }

type GeoRate struct {
	georate.GeoRate

	// Implement this flag when we need it
	TaxShipping bool `json:"taxShipping"`
	// Tax Name like 'Tax' or 'VAT'
	TaxName string `json:"taxName"`
}

type TaxRates struct {
	mixin.Model[TaxRates]

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

func (t TaxRates) Match(ctr, st, ct, pc string, c currency.Cents) (*GeoRate, int, int) {
	gr, level, i := georate.Match(t.GetGeoRates(), ctr, st, ct, pc, c)
	if gr != nil {
		return &t.GeoRates[i], level, i
	}

	return nil, level, i
}

func New(db *datastore.Datastore) *TaxRates {
	t := new(TaxRates)
	t.Init(db)
	t.GeoRates = make([]GeoRate, 0)
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("taxrates")
}
