package utils

import (
	aeds "appengine/datastore"

	"crowdstart.com/util/log"
)

// Helper to ignore tedious field mismatch errors (but warn appropriately
// during development)
func IgnoreFieldMismatch(err error) error {
	if err == nil {
		// Ignore nil error
		return nil
	}

	if _, ok := err.(*aeds.ErrFieldMismatch); ok {
		// Ignore any field mismatch errors, but warn user (at least during development)
		log.Warn("Ignoring, %v", err)
		return nil
	}

	// Any other errors we damn well need to know about!
	return err
}
