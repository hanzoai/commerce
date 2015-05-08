package ae

import (
	"crowdstart.com/util/log"
	"crowdstart.com/util/test/ae/aetest"
	"crowdstart.com/util/test/ae/appenginetesting"
	"crowdstart.com/util/test/ae/context"
	"crowdstart.com/util/test/ae/options"
)

// aliased for simplicity
type Context context.Context
type Options options.Options

func NewContext(opts ...Options) Context {
	var (
		_opts options.Options
		ctx   Context
		err   error
	)

	// Parse options
	switch len(opts) {
	case 0:
		_opts = _opts
	case 1:
		_opts = options.Options(opts[0])
	default:
		log.Panic("At most one ae.Options argument may be supplied.")
	}

	// Detect backend to use and create context
	backendUsed := "aetest"
	if _opts.PreferAppengineTesting || len(_opts.TaskQueues) > 0 {
		backendUsed = "appenginetesting"
		ctx, err = appenginetesting.New(_opts)
	} else {
		ctx, err = aetest.New(_opts)
	}

	// Blow up if we couldn't get a context.
	if err != nil {
		log.Panic("Failed to create %v context: %v", backendUsed, err)
	}

	return ctx
}
