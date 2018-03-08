package oauthtoken

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
)

func (t *Token) Load(properties []aeds.Property) error {
	// Ensure we're initialized
	t.Defaults()

	// Load supported properties
	err := datastore.LoadStruct(t, properties)
	if err != nil {
		return err
	}

	return err
}

func (t *Token) Save() ([]aeds.Property, error) {
	// Save properties
	return datastore.SaveStruct(t)
}
