package inventorylevel

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type InventoryLevel struct {
	mixin.BaseModel

	InventoryItemId  string `json:"inventoryItemId"`
	LocationId       string `json:"locationId"`
	StockedQuantity  int    `json:"stockedQuantity"`
	ReservedQuantity int    `json:"reservedQuantity"`
	IncomingQuantity int    `json:"incomingQuantity"`

	// Arbitrary key/value pairs
	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

// AvailableQuantity returns stock minus reserved.
func (l InventoryLevel) AvailableQuantity() int {
	return l.StockedQuantity - l.ReservedQuantity
}

func (l *InventoryLevel) Load(ps []datastore.Property) (err error) {
	// Prevent duplicate deserialization
	if l.Loaded() {
		return nil
	}

	// Ensure we're initialized
	l.Defaults()

	// Load supported properties
	if err = datastore.LoadStruct(l, ps); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(l.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(l.Metadata_), &l.Metadata)
	}

	return err
}

func (l *InventoryLevel) Save() ([]datastore.Property, error) {
	// Serialize unsupported properties
	l.Metadata_ = string(json.EncodeBytes(&l.Metadata))

	// Save properties
	return datastore.SaveStruct(l)
}
