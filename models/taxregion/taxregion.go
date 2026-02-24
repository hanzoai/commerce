package taxregion

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/types"
)

func init() { orm.Register[TaxRegion]("taxregion") }

type TaxRegion struct {
	mixin.Model[TaxRegion]

	CountryCode  string `json:"countryCode"`
	ProvinceCode string `json:"provinceCode"`
	ParentId     string `json:"parentId"`
	ProviderId   string `json:"providerId"`

	// Arbitrary key/value pairs associated with this tax region
	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (t *TaxRegion) Load(ps []datastore.Property) (err error) {
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

func New(db *datastore.Datastore) *TaxRegion {
	t := new(TaxRegion)
	t.Init(db)
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("taxregion")
}
