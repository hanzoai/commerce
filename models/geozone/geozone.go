package geozone

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type GeoZone struct {
	mixin.Model

	Type             string `json:"type"` // "country", "province", "city", "zip"
	CountryCode      string `json:"countryCode"`
	ProvinceCode     string `json:"provinceCode"`
	City             string `json:"city"`
	PostalExpression string `json:"postalExpression"`
	ServiceZoneId    string `json:"serviceZoneId"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (g *GeoZone) Load(ps []datastore.Property) (err error) {
	g.Defaults()

	if err = datastore.LoadStruct(g, ps); err != nil {
		return err
	}

	if len(g.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(g.Metadata_), &g.Metadata)
	}

	return err
}

func (g *GeoZone) Save() ([]datastore.Property, error) {
	g.Metadata_ = string(json.EncodeBytes(&g.Metadata))

	return datastore.SaveStruct(g)
}
