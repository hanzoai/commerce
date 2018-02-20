package ae

import (
	"strings"
	"time"

	"google.golang.org/appengine/aetest"

	"hanzo.io/util/rand"
)

// Aliased for simplicity
type Options *aetest.Options

// Generate random appId so dev servers can spin up in parallel
func appId() string {
	id := strings.Replace(strings.ToLower(rand.ShortId()), "_", "", -1)
	return "development-" + id

}

// Generate safe defaults
func defaults() Options {
	return &aetest.Options{
		AppID: appId(),
		StronglyConsistentDatastore: true,
		StartupTimeout:              time.Second * 60,
	}
}
