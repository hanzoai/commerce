package invoice

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func (i *Invoice) Load(properties []aeds.Property) (err error) {
	// Ensure we're initialized
	i.Defaults()

	// Load supported properties
	if err = datastore.LoadStruct(i, properties); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(i.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(i.Metadata_), &i.Metadata)
	}

	return err
}

func (i *Invoice) Save() ([]aeds.Property, error) {
	// Serialize unsupported properties
	i.Metadata_ = string(json.EncodeBytes(&i.Metadata))

	// Save properties
	return datastore.SaveStruct(i)
}
