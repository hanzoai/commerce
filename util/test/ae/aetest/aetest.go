package aetest

import (
	"github.com/zeekay/aetest"

	"crowdstart.com/util/test/ae/context"
	"crowdstart.com/util/test/ae/options"
)

// Create a new *aetest.Context
func New(opts options.Options) (context.Context, error) {
	opts.SetDefaults()

	_opts := &aetest.Options{
		StronglyConsistentDatastore: !opts.DisableStrongConsistency,
	}
	if ctx, err := aetest.NewContext(_opts); err != nil {
		return nil, err
	} else {
		_ctx := new(shimContext)
		_ctx.Context = ctx
		return _ctx, err
	}

}
