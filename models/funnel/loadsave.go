package funnel

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func (f *Funnel) Load(properties []aeds.Property) (err error) {
	// Ensure we're initialized
	f.Defaults()

	// Load supported properties
	err = datastore.LoadStruct(f, properties)
	if err != nil {
		return err
	}

	// Deserialize from datastore
	if len(f.Events_) > 0 {
		err = json.DecodeBytes([]byte(f.Events_), &f.Events)
	}

	return
}

func (f *Funnel) Save() ([]aeds.Property, error) {
	// Serialize unsupported properties
	f.Events_ = string(json.EncodeBytes(&f.Events))

	// Save properties
	return datastore.SaveStruct(f)
}
