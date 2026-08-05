package log

import (
	"context"

	"github.com/op/go-logging"
	"github.com/zap-proto/zip"
)

// Custom logger
type Logger struct {
	logging.Logger
	backend          *Backend
	verbose          bool
	verboseRequested bool
}

func (l *Logger) SetVerbose(verbose bool) {
	l.verbose = verbose
}

func (l *Logger) Verbose() bool {
	return l.verboseRequested || std.verbose
}

// Check if we have been passed a request context
func (l *Logger) detectContext(ctx interface{}) {
	l.verboseRequested = false

	switch ctx := ctx.(type) {
	case *zip.Ctx:
		// Get request context from locals (set by RequestContext middleware),
		// falling back to the fiber request context.
		if reqCtx := ctx.Context(); reqCtx != nil {
			l.backend.context = reqCtx.(context.Context)
		} else {
			l.backend.context = ctx.Context()
		}
		// The "verbose" key is only set by middleware/overrides; routes that
		// don't run it (e.g. the gateway/IAM-authenticated billing handlers)
		// would otherwise panic here on MustGet. Default to false when absent.
		if v := ctx.Locals("verbose"); v != nil {
			if b, ok := v.(bool); ok {
				l.verboseRequested = b
			}
		}

		// Request URI is useful for logging
		l.backend.requestURI = ctx.Fiber().OriginalURL()
	case context.Context:
		l.backend.context = ctx
	default:
		l.backend.context = nil
	}
}

// Check if error was passed as last argument
func (l *Logger) detectError(args []interface{}) {
	if len(args) > 0 {
		if err, ok := args[len(args)-1].(error); ok {
			l.backend.error = err
		}
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

// Process args, setting context if passed one.
func (l *Logger) parseArgs(args ...interface{}) []interface{} {
	if len(args) == 0 {
		return args
	}

	// Check if we have been passed a Gin context
	l.detectContext(args[len(args)-1])

	// Remove context from args if we were passed one
	if l.backend.context != nil {
		args = args[:len(args)-1]
	}

	// Last non-context argument might be an error.
	l.detectError(args)

	return args
}
