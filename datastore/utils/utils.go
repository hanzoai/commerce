package utils

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/util/log"
)

// Helper to ignore tedious field mismatch errors (but warn appropriately
// during development)
func IgnoreFieldMismatch(err error) error {
	if err == nil {
		return nil
	}

	if _, ok := err.(*aeds.ErrFieldMismatch); ok {
		log.Warn("Ignoring, %v", err)
		return nil
	}

	return err
}

// Completely ignore them even during development
func ReallyIgnoreFieldMismatch(err error) error {
	if err == nil {
		return nil
	}

	if _, ok := err.(*aeds.ErrFieldMismatch); ok {
		log.Warn("Ignoring, %v", err)
		return nil
	}

	return err
}
