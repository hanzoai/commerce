package coupon

import (
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
)

type Redemption struct {
	mixin.BaseModel

	// Coupon code (need not be unique).
	Code string `json:"code"`
}

func (r *Redemption) Load(props []datastore.Property) (err error) {
	// Load supported properties
	return datastore.LoadStruct(r, props)
}

func (r *Redemption) Save() (props []datastore.Property, err error) {
	r.Code = strings.ToUpper(r.Code)

	// Save properties
	return datastore.SaveStruct(r)
}
