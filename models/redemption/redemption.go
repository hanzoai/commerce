package coupon

import (
	"strings"

	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Redemption struct {
	mixin.Model

	// Coupon code (need not be unique).
	Code string `json:"code"`
}

func (r *Redemption) Load(c <-chan aeds.Property) (err error) {
	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(r, c)); err != nil {
		return err
	}

	return err
}

func (r *Redemption) Save(c chan<- aeds.Property) (err error) {

	r.Code = strings.ToUpper(r.Code)

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(r, c))
}
