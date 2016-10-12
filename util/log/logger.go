package log

import (
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"

	"appengine"
)

// Custom logger
type Logger struct {
	logging.Logger
	backend         *Backend
	verbose         bool
	verboseOverride bool
}

func (l *Logger) SetVerbose(verbose bool) {
	l.verbose = verbose
}

func (l *Logger) Verbose() bool {
	return l.verbose
}

func (l *Logger) VerboseOverride() bool {
	return l.verboseOverride
}

// Check if we've been pased a gin or app engine context
func (l *Logger) detectContext(ctx interface{}) {
	l.verboseOverride = false

	switch ctx := ctx.(type) {
	case *gin.Context:
		// Get App Engine from session
		l.backend.context = ctx.MustGet("appengine").(appengine.Context)
		l.verboseOverride = ctx.MustGet("verbose").(bool)

		// Request URI is useful for logging
		if ctx.Request != nil {
			l.backend.requestURI = ctx.Request.RequestURI
		}
	case appengine.Context:
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

// Process args, setting app engine context if passed one.
func (l *Logger) parseArgs(args ...interface{}) []interface{} {
	if len(args) == 0 {
		return args
	}

	// Check if we've been passed an App Engine or Gin context
	l.detectContext(args[len(args)-1])

	// Remove context from args if we were passed one
	if l.backend.context != nil {
		args = args[:len(args)-1]
	}

	// Last non-context argument might be an error.
	l.detectError(args)

	return args
}
