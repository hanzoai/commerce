package taxregion

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type TaxRegion struct {
	mixin.BaseModel

	CountryCode  string `json:"countryCode"`
	ProvinceCode string `json:"provinceCode"`
	ParentId     string `json:"parentId"`
	ProviderId   string `json:"providerId"`

	// Arbitrary key/value pairs associated with this tax region
	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (t *TaxRegion) Load(ps []datastore.Property) (err error) {
	t.Defaults()

	if err = datastore.LoadStruct(t, ps); err != nil {
		return err
	}

	if len(t.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(t.Metadata_), &t.Metadata)
	}

	return err
}

func (t *TaxRegion) Save() ([]datastore.Property, error) {
	t.Metadata_ = string(json.EncodeBytes(&t.Metadata))

	return datastore.SaveStruct(t)
}
