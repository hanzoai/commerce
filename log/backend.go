package log

import (
	"context"
	"log"

	aelog "google.golang.org/appengine/log"

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
	return b.logToAppEngine(level, formatted)
}

// Log implementation that uses App Engine's logging methods
func (b Backend) logToAppEngine(level logging.Level, formatted string) error {
	log.Println(formatted)
	switch level {
	case logging.WARNING:
		aelog.Warningf(b.context, formatted)
	case logging.ERROR:
		aelog.Errorf(b.context, formatted)
	case logging.CRITICAL:
		aelog.Criticalf(b.context, formatted)
	case logging.INFO:
		aelog.Infof(b.context, formatted)
	default:
		aelog.Debugf(b.context, formatted)
	}

	return nil
}

// Log method that customizes logging behavior for AppEngine dev server / production
func (b Backend) Log(level logging.Level, calldepth int, record *logging.Record) error {
	// Create formatted log output
	formatted := record.Formatted(calldepth + 2)

	if config.IsDevelopment || config.IsTest {
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
