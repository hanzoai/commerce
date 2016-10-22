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

func NewContext(args ...Options) Context {
	var (
		opts options.Options
		ctx  Context
		err  error
	)

	// Parse options
	switch len(args) {
	case 0:
		opts = opts
	case 1:
		opts = options.Options(args[0])
	default:
		log.Panic("At most one ae.Options argument may be supplied.")
	}

	// Detect backend to use and create context
	backendUsed := "aetest"
	if opts.PreferAppengineTesting || len(opts.TaskQueues) > 0 {
		log.Debug("Using appenginetesting backend")
		backendUsed = "appenginetesting"
		ctx, err = appenginetesting.New(opts)
	} else {
		log.Debug("Using aetest backend")
		ctx, err = aetest.New(opts)
	}

	// Blow up if we couldn't get a context.
	if err != nil {
		log.Panic("Failed to create %v context: %v", backendUsed, err)
	}

	return ctx
}
