package coupon

import (
	"strings"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
)

func (co *Coupon) Load(properties []aeds.Property) (err error) {
	// Load supported properties
	return aeds.LoadStruct(co, properties)
}

func (co *Coupon) Save() ([]aeds.Property, error) {
	co.Code = strings.ToUpper(co.Code)

	// Save properties
	return datastore.SaveStruct(co)
}
