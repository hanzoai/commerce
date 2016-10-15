package log

import (
	"fmt"

	"appengine"

	"github.com/op/go-logging"

	"crowdstart.com/util/json"
	"crowdstart.com/util/spew"
)

// Create a new App Engine-aware logger
func New() *Logger {
	log := new(Logger)

	isDevAppServer := appengine.IsDevAppServer()

	// Backend that is appengine-aware
	backend := new(Backend)
	backend.isDevAppServer = isDevAppServer

	log.backend = backend
	log.SetVerbose(isDevAppServer)

	// Log formatters, color for dev, plain for production
	plainFormatter := MustStringFormatter("%{longfile} %{longfunc} %{message}")
	colorFormatter := MustStringFormatter("%{color}%{level:.5s} %{longfile} %{longfunc} %{color:reset}%{message}")

	// Use plain formatter for production logging, color for dev server
	defaultBackend := logging.NewBackendFormatter(backend, plainFormatter)
	if isDevAppServer {
		defaultBackend = logging.NewBackendFormatter(backend, colorFormatter)
	} else {

	}

	multiBackend := logging.SetBackend(defaultBackend)
	log.SetBackend(multiBackend)
	return log
}

var std = New()

func SetVerbose(verbose bool) {
	std.SetVerbose(verbose)
}

func Verbose() bool {
	return std.Verbose()
}

func Debug(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	if !std.VerboseOverride() || !std.Verbose() {
		return
	}

	switch v := formatOrError.(type) {
	case error:
		std.Debugf(errAndStack(v))
	case string:
		std.Debugf(v, args...)
	}
}

func Info(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	if !std.VerboseOverride() || !std.Verbose() {
		return
	}

	switch v := formatOrError.(type) {
	case error:
		std.Infof(errAndStack(v))
	case string:
		std.Infof(v, args...)
	}
}

func Warn(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	if !std.VerboseOverride() || !std.Verbose() {
		return
	}

	switch v := formatOrError.(type) {
	case error:
		std.Warningf(errAndStack(v))
	case string:
		std.Warningf(v, args...)
	}
}

func Error(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		std.Errorf(errAndStack(v))
	case string:
		std.Errorf(v, args...)
	}
}

func Fatal(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		std.Fatalf(errAndStack(v))
	case string:
		std.Fatalf(v, args...)
	}
}

func Panic(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		std.Panicf(errAndStack(v))
	case string:
		std.Panicf(v, args...)
	}
}

func Dump(formatOrObject interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	if !std.VerboseOverride() || !std.Verbose() {
		return
	}

	switch v := formatOrObject.(type) {
	case string:
		args, obj := std.dumpObject(args)
		msg := fmt.Sprintf(v, args...)
		dump := spew.Sdump(obj)
		std.Debugf("%s\n%s", msg, dump)
	default:
		dump := spew.Sdump(v)
		std.Debugf("\n%s", dump)
	}
}

func JSON(formatOrObject interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	if !std.VerboseOverride() || !std.Verbose() {
		return
	}

	switch v := formatOrObject.(type) {
	case string:
		args, obj := std.dumpObject(args)
		msg := fmt.Sprintf(v, args...)
		std.Debugf("%s\n%s", msg, json.Encode(obj))
	default:
		std.Debugf("\n%s", json.Encode(v))
	}
}

func Stack(args ...interface{}) {
	args = std.parseArgs(args...)

	if len(args) > 0 {
		format := args[0].(string)
		msg := fmt.Sprintf(format, args[1:]...)
		std.Debugf(msg + stack(4))
	} else {
		std.Debugf(stack(4))
	}
}
