package customergroup

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type CustomerGroup struct {
	mixin.Model

	Name string `json:"name"`

	// Arbitrary metadata
	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (g *CustomerGroup) Load(ps []datastore.Property) (err error) {
	g.Defaults()

	if err = datastore.LoadStruct(g, ps); err != nil {
		return err
	}

	if len(g.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(g.Metadata_), &g.Metadata)
	}

	return err
}

func (g *CustomerGroup) Save() ([]datastore.Property, error) {
	g.Metadata_ = string(json.EncodeBytes(&g.Metadata))

	return datastore.SaveStruct(g)
}
