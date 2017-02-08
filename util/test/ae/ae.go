package ae

import (
	"golang.org/x/net/context"

	"hanzo.io/util/log"
	"hanzo.io/util/test/ae/env"
	"hanzo.io/util/test/ae/options"

	"github.com/zeekay/aetest"
)

// aliased for simplicity
type Options options.Options
type Instance aetest.Instance

func NewContext(opts ...Options) (context.Context, Instance, error) {
	var (
		_opts options.Options
		ctx   context.Context
		inst  aetest.Instance
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

	ctx, inst, err = env.New(_opts)

	// Blow up if we couldn't get a context.
	if err != nil {
		log.Panic("Failed to create context: %v", err)
	}

	return ctx, inst, err
}
