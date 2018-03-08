package query

import (
	aeds "google.golang.org/appengine/datastore"
)

var (
	// Alias appengine types
	Done                 = aeds.Done
	ErrNoSuchEntity      = aeds.ErrNoSuchEntity
	ErrInvalidEntityType = aeds.ErrInvalidEntityType
	ErrInvalidKey        = aeds.ErrInvalidKey

	// Custom Errors
	InvalidKey  = ErrInvalidKey
	KeyNotFound = ErrNoSuchEntity
)
