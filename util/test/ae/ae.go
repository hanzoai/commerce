package ae

import (
	"crowdstart.io/util/test/ae/aetest"
	"crowdstart.io/util/test/ae/appenginetesting"
	"crowdstart.io/util/test/ae/context"
	"crowdstart.io/util/test/ae/options"
)

// aliased for simplicity
type Context context.Context

func NewContext(opts Options) (Context, error) {
	opts.SetDefaults()

	if len(opts.TaskQueues) > 0 {
		return appenginetesting.New(options.Options(opts))
	} else {
		return aetest.New(options.Options(opts))
	}
}
