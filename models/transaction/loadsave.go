package transaction

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
)

func (t *Transaction) Load(properties []aeds.Property) error {
	// Load supported properties
	return datastore.LoadStruct(t, properties)
}

func (t *Transaction) Save() ([]aeds.Property, error) {
	// Save properties
	return datastore.SaveStruct(t)
}
