package ae

import (
	"crowdstart.io/util/log"
	"crowdstart.io/util/test/ae/aetest"
	"crowdstart.io/util/test/ae/appenginetesting"
	"crowdstart.io/util/test/ae/context"
	"crowdstart.io/util/test/ae/options"
)

// aliased for simplicity
type Context context.Context

func NewContext(opts ...Options) Context {
	var opt Options
	var ctx Context
	var err error

	switch len(opts) {
	case 0:
		opt = Options{}
		opt.SetDefaults()
	case 1:
		opt = opts[0]
		opt.SetDefaults()
	default:
		log.Panic("At most one ae.Options argument may be supplied.")
	}

	if len(opt.TaskQueues) > 0 {
		ctx, err = appenginetesting.New(options.Options(opt))
	} else {
		ctx, err = aetest.New(options.Options(opt))
	}

	// Blow up if we couldn't get a context.
	if err != nil {
		log.Panic("Unable to get ae.Context: %v", err)
	}

	return ctx
}
