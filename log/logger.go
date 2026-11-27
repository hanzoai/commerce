package log

import (
	"context"
	"sync"

	"github.com/op/go-logging"
	"github.com/zap-proto/zip"
)

// Custom logger
type Logger struct {
	logging.Logger
	mu               sync.Mutex
	backend          *Backend
	verbose          bool
	verboseRequested bool
}

func (l *Logger) SetVerbose(verbose bool) {
	l.mu.Lock()
	l.verbose = verbose
	l.mu.Unlock()
}

func (l *Logger) Verbose() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.verboseRequested || std.verbose
}

// Check if we have been passed a request context
func (l *Logger) detectContext(ctx interface{}) {
	l.verboseRequested = false

	switch ctx := ctx.(type) {
	case *zip.Ctx:
		if reqCtx := ctx.Context(); reqCtx != nil {
			l.backend.context = reqCtx.(context.Context)
		} else {
			l.backend.context = ctx.Context()
		}
		if v := ctx.Locals("verbose"); v != nil {
			if b, ok := v.(bool); ok {
				l.verboseRequested = b
			}
		}
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
		last := args[len(args)-1]
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

	l.detectContext(args[len(args)-1])

	if l.backend.context != nil {
		args = args[:len(args)-1]
	}

	l.detectError(args)

	return args
}
