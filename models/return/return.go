package return_

import (
	"time"

	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/fulfillment"
	"hanzo.io/util/json"
	"hanzo.io/util/val"

	. "hanzo.io/models"
	"hanzo.io/models/lineitem"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Return struct {
	mixin.Model

	// Store this was sold from (if any)
	StoreId string `json:"storeId,omitempty"`

	// Associated Crowdstart user or buyer.
	UserId string `json:"userId,omitempty"`

	// Associated order ID, if any
	OrderId string `json:"orderId,omitempty"`

	// Individual line items
	Items  []lineitem.LineItem `json:"items" datastore:"-"`
	Items_ string              `json:"-" datastore:",noindex"`

	// Fulfillment information
	Fulfillment fulfillment.Fulfillment `json:"fulfillment"`

	// Series of events that have occured relevant to this order
	History []Event `json:"-,omitempty" datastore:",noindex"`

	// Make a custom string for this when we figure out the states...
	Status string

	// Arbitrary key/value pairs associated with this order
	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`

	CancelledAt time.Time `json:"cancelledAt"`
	CompletedAt time.Time `json:"completedAt"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
	ExpectedAt  time.Time `json:"expectedAt"`
	DeliveredAt time.Time `json:"deliveredAt"`
	PickedUpAt  time.Time `json:"pickedUpAt"`
	ProcessedAt time.Time `json:"processedAt"`
	ReturnedAt  time.Time `json:"returnedAt"`
	SubmittedAt time.Time `json:"submittedAt"`
}

func (c *Return) Validator() *val.Validator {
	return val.New()
}

func (c *Return) Load(ch <-chan aeds.Property) (err error) {
	// Prevent duplicate deserialization
	if c.Loaded() {
		return nil
	}

	// Ensure we're initialized
	c.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(c, ch)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(c.Items_) > 0 {
		err = json.DecodeBytes([]byte(c.Items_), &c.Items)
	}

	if len(c.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(c.Metadata_), &c.Metadata)
	}

	return err
}

func (c *Return) Save(ch chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	c.Metadata_ = string(json.EncodeBytes(&c.Metadata))
	c.Items_ = string(json.EncodeBytes(c.Items))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(c, ch))
}
