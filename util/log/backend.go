package log

import (
	"log"

	"appengine"

	"github.com/op/go-logging"

	"hanzo.io/config"
)

// Custom logger backend that knows about AppEngine
type Backend struct {
	context    context.Context
	error      error
	requestURI string
	verbose    bool
}

func (b Backend) Verbose() bool {
	return b.verbose
}

// Log implementation for local dev server only.
func (b Backend) logToDevServer(level logging.Level, formatted string) error {
	log.Println(formatted)
	return nil
}

// Log implementation that uses App Engine's logging methods
func (b Backend) logToAppEngine(level logging.Level, formatted string) error {
	switch level {
	case logging.WARNING:
		b.context.Warningf(formatted)
	case logging.ERROR:
		b.context.Errorf(formatted)
	case logging.CRITICAL:
		b.context.Criticalf(formatted)
	case logging.INFO:
		b.context.Infof(formatted)
	default:
		b.context.Debugf(formatted)
	}

	return nil
}

// Log method that customizes logging behavior for AppEngine dev server / production
func (b Backend) Log(level logging.Level, calldepth int, record *logging.Record) error {
	// Create formatted log output
	formatted := record.Formatted(calldepth + 2)

	if config.IsDevelopment {
		// Logging for local server
		return b.logToDevServer(level, formatted)
	} else {
		// Log to App Engine in staging and production when passed a context
		if b.context != nil {
			return b.logToAppEngine(level, formatted)
		}
	}
	return nil
}
