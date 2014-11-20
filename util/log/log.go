package log

import (
	"appengine"
	"github.com/zeekay/go-logging"
	"log"
)

// Custom logger
type Logger struct {
	logging.Logger
	appengineBackend *AppengineBackend
}

// Set app engine context if passed one
func (l *Logger) setContext(args ...interface{}) {
	ctx := args[len(args)-1]
	switch ctx := ctx.(type) {
	case appengine.Context:
		l.appengineBackend.context = ctx
	default:
		l.appengineBackend.context = nil
	}
}

// Custom logger backend that knows about AppEngine
type AppengineBackend struct {
	context appengine.Context
}

func (b AppengineBackend) Log(level logging.Level, calldepth int, record *logging.Record) error {
	formatted := record.Formatted(calldepth + 2)

	if b.context != nil {
		switch level {
		case logging.WARNING:
			b.context.Warningf(formatted)
		case logging.ERROR:
			b.context.Errorf(formatted)
		case logging.INFO:
			b.context.Infof(formatted)
		case logging.NOTICE, logging.DEBUG:
			b.context.Debugf(formatted)
		}
	} else {
		log.Println(formatted)
	}

	return nil
}

func New() *Logger {
	log := new(Logger)

	format := logging.MustStringFormatter(
		"%{color}%{level:.5s} %{shortfile} %{longfunc} %{color:reset}%{message}",
	)

	backend := new(AppengineBackend)
	log.appengineBackend = backend

	// For messages written to backend2 we want to add some additional
	// information to the output, including the used log level and the name of
	// the function.
	defaultBackend := logging.NewBackendFormatter(backend, format)

	// Only errors and more severe messages should be sent to backend1
	errorBackend := logging.AddModuleLevel(backend)
	errorBackend.SetLevel(logging.ERROR, "")

	// Set the backends to be used.
	multiBackend := logging.SetBackend(defaultBackend, errorBackend)

	log.SetBackend(multiBackend)

	return log
}

var std = New()

func Debug(format string, args ...interface{}) {
	std.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	std.Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	std.Warning(format, args...)
}

func Error(format string, args ...interface{}) {
	std.Error(format, args...)
}

func Fatal(format string, args ...interface{}) {
	std.Fatalf(format, args...)
}

func Panic(format string, args ...interface{}) {
	std.Panicf(format, args)
}
