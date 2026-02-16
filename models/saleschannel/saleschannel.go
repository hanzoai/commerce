package saleschannel

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type SalesChannel struct {
	mixin.Model

	// Name of sales channel
	Name string `json:"name"`

	// Description of sales channel
	Description string `json:"description" datastore:",noindex"`

	// Whether this sales channel is disabled
	IsDisabled bool `json:"isDisabled"`

	// Arbitrary metadata
	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (s *SalesChannel) Load(ps []datastore.Property) (err error) {
	s.Defaults()

	if err = datastore.LoadStruct(s, ps); err != nil {
		return err
	}

	if len(s.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(s.Metadata_), &s.Metadata)
	}

	return err
}

func (s *SalesChannel) Save() ([]datastore.Property, error) {
	s.Metadata_ = string(json.EncodeBytes(&s.Metadata))

	return datastore.SaveStruct(s)
}
