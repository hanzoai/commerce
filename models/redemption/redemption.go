package coupon

import (
	"strings"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

type Redemption struct {
	mixin.Model

	// Coupon code (need not be unique).
	Code string `json:"code"`
}

func (r *Redemption) Load(props []aeds.Property) (err error) {
	// Load supported properties
	return datastore.LoadStruct(r, props)
}

func (r *Redemption) Save() (props []aeds.Property, err error) {
	r.Code = strings.ToUpper(r.Code)

	// Save properties
	return datastore.SaveStruct(r)
}
