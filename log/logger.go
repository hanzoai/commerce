package log

import (
	"context"

	"github.com/op/go-logging"
	"github.com/zap-proto/zip"
)

// Logger is the process's logger. Everything on it is SET AT STARTUP and read
// by every goroutine after — there is no per-call state here, deliberately.
//
// There was: a request's context, its URI, its error and its verbose flag were
// written onto this one shared value on EVERY log call. One process serves
// every tenant concurrently, so those fields were a data race and, worse, a
// cross-request bleed — one request's URI decorating another's line. They are
// gone rather than locked: per-call facts belong to the call, and a value that
// travels with the call cannot be read by a request that did not make it.
type Logger struct {
	logging.Logger
	backend *Backend
	verbose bool
}

func (l *Logger) SetVerbose(verbose bool) {
	l.verbose = verbose
}

// Verbose reports whether this process logs debug output at all. Whether ONE
// call does is [call.verbose], which travels with that call.
func (l *Logger) Verbose() bool { return l.verbose }

// call is what one log call learned from its own arguments. It is a value: two
// concurrent calls have two of them, which is the whole point.
type call struct {
	// args are the format arguments with any trailing context removed.
	args []interface{}
	// verbose is true when the request that made this call asked for debug
	// output, whatever the process default is.
	verbose bool
}

// detectContext reports whether the last argument was a context — in which case
// it is not a format argument — and whether that context asked for verbose.
func detectContext(arg interface{}) (isContext, verbose bool) {
	switch ctx := arg.(type) {
	case *zip.Ctx:
		// The "verbose" key is only set by middleware/overrides; routes that
		// don't run it (e.g. the gateway/IAM-authenticated billing handlers)
		// would otherwise panic here on MustGet. Default to false when absent.
		if v := ctx.Locals("verbose"); v != nil {
			if b, ok := v.(bool); ok {
				verbose = b
			}
		}
		return true, verbose
	case context.Context:
		return true, false
	default:
		return false, false
	}
}

// Grab last object (presumably to dump)
func (l *Logger) dumpObject(args []interface{}) ([]interface{}, interface{}) {
	if len(args) > 0 {
		// Grab last argument
		last := args[len(args)-1]
		// Remove from args
		args = args[:len(args)-1]
		return args, last
	}
	return args, nil
}

// parseArgs reads one call's arguments and reports what they mean, TOUCHING
// NOTHING SHARED. A trailing request context is a context and not a format
// argument, so it comes off; whether that context asked for verbose travels
// back with the call rather than onto the logger.
func (l *Logger) parseArgs(args ...interface{}) call {
	if len(args) == 0 {
		return call{args: args, verbose: l.verbose}
	}
	isContext, requested := detectContext(args[len(args)-1])
	if isContext {
		args = args[:len(args)-1]
	}
	return call{args: args, verbose: requested || l.verbose}
}
