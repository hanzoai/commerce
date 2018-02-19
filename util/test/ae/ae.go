package ae

import (
	"golang.org/x/net/context"

	"hanzo.io/log"
	"hanzo.io/util/test/ae/aetest"
	"hanzo.io/util/test/ae/options"
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

	ctx, err = aetest.New(opts)

	// Blow up if we couldn't get a context.
	if err != nil {
		log.Panic("Failed to create context: %v", err)
	}

	return ctx
}
