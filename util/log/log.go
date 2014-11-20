package log

import (
	"github.com/zeekay/go-logging"
	"log"
)

type Logger struct {
	logging.Logger
}

type LogBackend struct{}

func (b LogBackend) Log(level logging.Level, calldepth int, record *logging.Record) error {
	log.Println(record.Formatted(calldepth + 2))
	return nil
}

func New() *Logger {
	log := new(Logger)

	format := logging.MustStringFormatter(
		"%{color}%{level:.5s} %{shortfile} %{longfunc} %{color:reset}%{message}",
	)

	backend := new(LogBackend)

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
	std.Debug(format, args...)
}

func Fatal(format string, args ...interface{}) {
	std.Fatalf(format, args...)
}

func Panic(format string, args ...interface{}) {
	std.Panicf(format, args)
}
