package webhook

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func (s *Webhook) Load(properties []aeds.Property) (err error) {
	// Ensure we're initialized
	s.Defaults()

	// Load supported properties
	err = datastore.LoadStruct(s, properties)
	if err != nil {
		return err
	}

	// Deserialize from datastore
	if len(s.Events_) > 0 {
		err = json.DecodeBytes([]byte(s.Events_), &s.Events)
	}

	return err
}

func (s *Webhook) Save() ([]aeds.Property, error) {
	// Serialize unsupported properties
	s.Events_ = string(json.EncodeBytes(&s.Events))

	// Save properties
	return datastore.SaveStruct(s)
}
