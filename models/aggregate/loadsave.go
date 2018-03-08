package aggregate

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
)

func (a *Aggregate) Load(properties []aeds.Property) (err error) {
	// Ensure we're initialized
	a.Defaults()

	// Load supported properties
	return datastore.LoadStruct(a, properties)
}

func (a *Aggregate) Save() ([]aeds.Property, error) {
	// Save properties
	return datastore.SaveStruct(a)
}
