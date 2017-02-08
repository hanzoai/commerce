package funnel

import (
	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/util/json"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Funnel struct {
	mixin.Model

	Name    string     `json:"name"`
	Events  [][]string `json:"events" datastore:"-"`
	Events_ string     `json:"-"`
}

func (f *Funnel) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	f.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(f, c)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(f.Events_) > 0 {
		err = json.DecodeBytes([]byte(f.Events_), &f.Events)
	}

	return
}

func (f *Funnel) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	f.Events_ = string(json.EncodeBytes(&f.Events))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(f, c))
}
