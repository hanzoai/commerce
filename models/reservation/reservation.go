package reservation

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type ReservationItem struct {
	mixin.Model

	InventoryItemId string `json:"inventoryItemId"`
	LocationId      string `json:"locationId"`
	LineItemId      string `json:"lineItemId"`
	Quantity        int    `json:"quantity"`
	AllowBackorder  bool   `json:"allowBackorder"`
	Description     string `json:"description"`
	ExternalId      string `json:"externalId"`

	// Arbitrary key/value pairs
	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (r *ReservationItem) Load(ps []datastore.Property) (err error) {
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

func (r *ReservationItem) Save() ([]datastore.Property, error) {
	// Serialize unsupported properties
	r.Metadata_ = string(json.EncodeBytes(&r.Metadata))

	// Save properties
	return datastore.SaveStruct(r)
}
