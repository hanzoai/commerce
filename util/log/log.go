package log

import (
	"fmt"
	"log"
	"runtime/debug"
	"strings"

	"appengine"

	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"

	"crowdstart.com/util/json"
	"crowdstart.com/util/spew"
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

// Custom logger backend that knows about AppEngine
type Backend struct {
	context        appengine.Context
	error          error
	requestURI     string
	verbose        bool
	isDevAppServer bool
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

	if b.isDevAppServer {
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

	if !std.VerboseOverride() && !std.Verbose() {
		return
	}

	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		std.Debugf("%s", args...)
	case string:
		std.Debugf(v, args...)
	}
}

func Info(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		std.Infof("%s", args...)
	case string:
		std.Infof(v, args...)
	}
}

func Warn(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		std.Warningf("%s", args...)
	case string:
		std.Warningf(v, args...)
	}
}

func Error(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		std.Errorf("%s", args...)
	case string:
		std.Errorf(v, args...)
	}
}

func Fatal(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		std.Fatalf("%s", args...)
	case string:
		std.Fatalf(v, args...)
	}
}

func Panic(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		std.Panicf("%s", args...)
	case string:
		std.Panicf(v, args...)
	}
}

func Dump(formatOrObject interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

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

	switch v := formatOrObject.(type) {
	case string:
		args, obj := std.dumpObject(args)
		msg := fmt.Sprintf(v, args...)
		std.Debugf("%s\n%s", msg, json.Encode(obj))
	default:
		std.Debugf("\n%s", json.Encode(v))
	}
}

func Stack() {
	stack := debug.Stack()
	lines := strings.Split(string(stack), "\n")
	trace := []string{""}
	for i := 4; i < len(lines); i++ {
		if strings.Contains(lines[i], "github.com/onsi/ginkgo") {
			break
		}
		trace = append(trace, lines[i])
	}
	std.Debug(strings.Join(trace, "\n"))
}
