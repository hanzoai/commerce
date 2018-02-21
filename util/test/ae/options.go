package ae

import "google.golang.org/appengine/aetest"

// Aliased for simplicity
type Options *aetest.Options

// Generate safe defaults
func defaults() Options {
	return &aetest.Options{
		AppID: "None",
		StronglyConsistentDatastore: true,
	}
}
