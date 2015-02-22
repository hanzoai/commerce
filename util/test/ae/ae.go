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

	if _opts.PreferAppengineTesting || len(_opts.TaskQueues) > 0 {
		ctx, err = appenginetesting.New(_opts)
	} else {
		ctx, err = aetest.New(_opts)
	}

	// Blow up if we couldn't get a context.
	if err != nil {
		log.Panic("Unable to get ae.Context: %v", err)
	}

	return ctx
}
