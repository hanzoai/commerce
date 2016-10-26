package aetest

import (
	"github.com/zeekay/aetest"

	"crowdstart.com/util/log"
	"crowdstart.com/util/test/ae/context"
	"crowdstart.com/util/test/ae/options"
)

// Create a new *aetest.Context
func New(opts options.Options) (context.Context, error) {
	opts.SetDefaults()

	aeopts := &aetest.Options{
		StronglyConsistentDatastore: !opts.DisableStrongConsistency,
	}

	log.Debug("Creating new aetest context: %#v", aeopts)

	if aectx, err := aetest.NewContext(aeopts); err != nil {
		return nil, err
	} else {
		ctx := new(shimContext)
		ctx.Context = aectx
		return ctx, err
	}

}
