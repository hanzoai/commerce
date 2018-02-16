package aetest

import (
	"golang.org/x/net/context"
	"google.golang.org/appengine/aetest"

	"hanzo.io/util/log"
	"hanzo.io/util/test/ae/options"
)

// Create a new *aetest.Context
func New(opts options.Options) (context.Context, error) {
	opts.SetDefaults()

	aeopts := &aetest.Options{
		AppID: opts.AppId,
		// StartupTimeout: time.Duration
		StronglyConsistentDatastore: !opts.DisableStrongConsistency,
	}

	log.Debug("Creating new aetest context: %#v", aeopts)

	ctx, done, err := aetest.NewContext()
	if err != nil {
		return nil, err
	}
	defer done()

	return ctx, nil
}
