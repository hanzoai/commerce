package subscription

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func (s *Subscription) Load(properties []aeds.Property) (err error) {
	// Ensure we're initialized
	s.Defaults()

	// Load supported properties
	if err = datastore.LoadStruct(s, properties); err != nil {
		return err
	}

	// Set order number
	s.Number = s.NumberFromId()

	// Deserialize from datastore
	if len(s.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(s.Metadata_), &s.Metadata)
	}

	return err
}

func (s *Subscription) Save() ([]aeds.Property, error) {
	// Serialize unsupported properties
	s.Metadata_ = string(json.EncodeBytes(&s.Metadata))

	// Save properties
	return datastore.SaveStruct(s)
}
