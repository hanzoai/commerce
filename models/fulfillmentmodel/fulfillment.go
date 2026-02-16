package fulfillmentmodel

import (
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type FulfillmentLabel struct {
	TrackingNumber string `json:"trackingNumber"`
	TrackingUrl    string `json:"trackingUrl"`
	LabelUrl       string `json:"labelUrl"`
}

type FulfillmentItem struct {
	Title           string `json:"title"`
	SKU             string `json:"sku"`
	Quantity        int    `json:"quantity"`
	LineItemId      string `json:"lineItemId"`
	InventoryItemId string `json:"inventoryItemId"`
}

type Fulfillment struct {
	mixin.Model

	OrderId          string     `json:"orderId"`
	ShippingOptionId string     `json:"shippingOptionId"`
	ProviderId       string     `json:"providerId"`
	LocationId       string     `json:"locationId"`
	PackedAt         *time.Time `json:"packedAt,omitempty"`
	ShippedAt        *time.Time `json:"shippedAt,omitempty"`
	DeliveredAt      *time.Time `json:"deliveredAt,omitempty"`
	CanceledAt       *time.Time `json:"canceledAt,omitempty"`

	Items  []FulfillmentItem `json:"items" datastore:"-"`
	Items_ string            `json:"-" datastore:",noindex"`

	Labels  []FulfillmentLabel `json:"labels" datastore:"-"`
	Labels_ string             `json:"-" datastore:",noindex"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (f *Fulfillment) Load(ps []datastore.Property) (err error) {
	f.Defaults()

	if err = datastore.LoadStruct(f, ps); err != nil {
		return err
	}

	if len(f.Items_) > 0 {
		err = json.DecodeBytes([]byte(f.Items_), &f.Items)
	}

	if len(f.Labels_) > 0 {
		err = json.DecodeBytes([]byte(f.Labels_), &f.Labels)
	}

	if len(f.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(f.Metadata_), &f.Metadata)
	}

	return err
}

func (f *Fulfillment) Save() ([]datastore.Property, error) {
	f.Items_ = string(json.EncodeBytes(f.Items))
	f.Labels_ = string(json.EncodeBytes(f.Labels))
	f.Metadata_ = string(json.EncodeBytes(&f.Metadata))

	return datastore.SaveStruct(f)
}
