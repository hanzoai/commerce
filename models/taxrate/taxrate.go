package taxrate

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type TaxRate struct {
	mixin.Model

	Rate         float64 `json:"rate"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	IsDefault    bool    `json:"isDefault"`
	IsCombinable bool    `json:"isCombinable"`
	TaxRegionId  string  `json:"taxRegionId"`

	// Arbitrary key/value pairs associated with this tax rate
	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (t *TaxRate) Load(ps []datastore.Property) (err error) {
	t.Defaults()

	if err = datastore.LoadStruct(t, ps); err != nil {
		return err
	}

	if len(t.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(t.Metadata_), &t.Metadata)
	}

	return err
}

func (t *TaxRate) Save() ([]datastore.Property, error) {
	t.Metadata_ = string(json.EncodeBytes(&t.Metadata))

	return datastore.SaveStruct(t)
}
