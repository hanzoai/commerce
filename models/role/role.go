package role

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type Role struct {
	mixin.Model

	Name string `json:"name"`

	// Permissions stored as JSON in datastore
	Permissions  []string `json:"permissions" datastore:"-"`
	Permissions_ string   `json:"-" datastore:",noindex"`

	// Arbitrary metadata
	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (r *Role) Load(ps []datastore.Property) (err error) {
	r.Defaults()

	if err = datastore.LoadStruct(r, ps); err != nil {
		return err
	}

	if len(r.Permissions_) > 0 {
		if err = json.DecodeBytes([]byte(r.Permissions_), &r.Permissions); err != nil {
			return err
		}
	}

	if len(r.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(r.Metadata_), &r.Metadata)
	}

	return err
}

func (r *Role) Save() ([]datastore.Property, error) {
	r.Permissions_ = string(json.EncodeBytes(&r.Permissions))
	r.Metadata_ = string(json.EncodeBytes(&r.Metadata))

	return datastore.SaveStruct(r)
}
