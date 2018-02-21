package funnel

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/util/json"
)

type Funnel struct {
	mixin.Model

	Name    string     `json:"name"`
	Events  [][]string `json:"events" datastore:"-"`
	Events_ string     `json:"-"`
}

func (f *Funnel) Load(ps []aeds.Property) (err error) {
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

func (f *Funnel) Save() (ps []aeds.Property, err error) {
	// Serialize unsupported properties
	f.Events_ = string(json.EncodeBytes(&f.Events))

	// Save properties
	return datastore.SaveStruct(f)
}
