package review

import (
	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/util/json"

	. "hanzo.io/models"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Review struct {
	mixin.Model

	UserId string `json:"userId"`

	ProductId string `json:"productId"`
	VariantId string `json:"variantId"`

	Name    string `json:"name"`
	Device  string `json:"device"`
	Comment string `json:"comment"`
	Rating  int    `json:"rating"`

	Enabled bool `json:"-"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (r *Review) Load(c <-chan aeds.Property) (err error) {
	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(r, c)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(r.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(r.Metadata_), &r.Metadata)
	}

	return
}

func (r *Review) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	r.Metadata_ = string(json.EncodeBytes(&r.Metadata))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(r, c))
}
