package inventory

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type InventoryItem struct {
	mixin.BaseModel

	SKU              string `json:"sku"`
	OriginCountry    string `json:"originCountry"`
	HSCode           string `json:"hsCode"`
	MidCode          string `json:"midCode"`
	Material         string `json:"material"`
	Weight           int    `json:"weight"`
	Length           int    `json:"length"`
	Height           int    `json:"height"`
	Width            int    `json:"width"`
	RequiresShipping bool   `json:"requiresShipping"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Thumbnail        string `json:"thumbnail"`

	// Arbitrary key/value pairs
	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (r *InventoryItem) Load(ps []datastore.Property) (err error) {
	// Prevent duplicate deserialization
	if r.Loaded() {
		return nil
	}

	// Ensure we're initialized
	r.Defaults()

	// Load supported properties
	if err = datastore.LoadStruct(r, ps); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(r.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(r.Metadata_), &r.Metadata)
	}

	return err
}

func (r *InventoryItem) Save() ([]datastore.Property, error) {
	// Serialize unsupported properties
	r.Metadata_ = string(json.EncodeBytes(&r.Metadata))

	// Save properties
	return datastore.SaveStruct(r)
}
