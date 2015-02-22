package aetest

import (
	"github.com/zeekay/aetest"

	"crowdstart.io/util/test/ae/context"
	"crowdstart.io/util/test/ae/options"
)

// Create a new *aetest.Context
func New(opts options.Options) (context.Context, error) {
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
