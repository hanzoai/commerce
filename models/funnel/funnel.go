package funnel

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"
)

type Funnel struct {
	mixin.Model

	Name    string     `json:"name"`
	Events  [][]string `json:"events" datastore:"-"`
	Events_ string     `json:"-"`
}

func (f *Funnel) Load(ps []datastore.Property) (err error) {
	// Ensure we're initialized
	f.Defaults()

	// Load supported properties
	if err = datastore.LoadStruct(f, ps); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(f.Events_) > 0 {
		err = json.DecodeBytes([]byte(f.Events_), &f.Events)
	}

	return
}

func (f *Funnel) Save() (ps []datastore.Property, err error) {
	// Serialize unsupported properties
	f.Events_ = string(json.EncodeBytes(&f.Events))

	// Save properties
	return datastore.SaveStruct(f)
}
