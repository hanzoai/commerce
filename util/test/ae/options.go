package ae

import (
	"time"

	"google.golang.org/appengine/aetest"
)

// Aliased for simplicity
type Options *aetest.Options

// Generate safe defaults
func defaults() Options {
	return &aetest.Options{
		AppID: "development",
		StronglyConsistentDatastore: true,
		StartupTimeout:              time.Second * 120,
	}
}
