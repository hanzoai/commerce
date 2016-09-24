package log

import (
	"log"
	"strings"

	"appengine"

	"github.com/gin-gonic/gin"
	"github.com/zeekay/go-logging"

	"github.com/davecgh/go-spew/spew"
)

// Custom logger
type Logger struct {
	logging.Logger
	appengineBackend *AppengineBackend
	verbose          bool
	verboseOverride  bool
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
		l.appengineBackend.context = ctx.MustGet("appengine").(appengine.Context)
		l.verboseOverride = ctx.MustGet("verbose").(bool)

		// Request URI is useful for logging
		if ctx.Request != nil {
			l.appengineBackend.requestURI = ctx.Request.RequestURI
		}
	case appengine.Context:
		l.appengineBackend.context = ctx
	default:
		l.appengineBackend.context = nil
	}
}

// Check if error was passed as last argument
func (l *Logger) detectError(args []interface{}) {
	if len(args) > 0 {
		if err, ok := args[len(args)-1].(error); ok {
			l.appengineBackend.error = err
		}
	}
}

// Process args, setting app engine context if passed one.
func (l *Logger) parseArgs(args ...interface{}) []interface{} {
	if len(args) == 0 {
		return args
	}

	// Check if we've been passed an App Engine or Gin context
	l.detectContext(args[len(args)-1])

	// Remove context from args if we were passed one
	if l.appengineBackend.context != nil {
		args = args[:len(args)-1]
	}

	// Last non-context argument might be an error.
	l.detectError(args)

	return args
}

// Custom logger backend that knows about AppEngine
type AppengineBackend struct {
	context    appengine.Context
	error      error
	requestURI string
	verbose    bool
}

func (b AppengineBackend) Verbose() bool {
	return b.verbose
}

// Log implementation for local dev server only.
func (b AppengineBackend) logToDevServer(level logging.Level, formatted string) error {
	if level == logging.INFO {
		// Hack to make INFO level less verbose
		parts := strings.Split(formatted, " ")
		parts = append([]string{"INFO"}, parts[3:]...)
		formatted = strings.Join(parts, " ")
	}

	log.Println(formatted)

	return nil
}

// Log implementation that uses App Engine's logging methods
func (b AppengineBackend) logToAppEngine(level logging.Level, formatted string) error {
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
func (b AppengineBackend) Log(level logging.Level, calldepth int, record *logging.Record) error {
	// Create formatted log output
	formatted := record.Formatted(calldepth + 2)

	// Log using App Engine backend if we have a context, otherwise dev server
	if b.context != nil {
		return b.logToAppEngine(level, formatted)
	} else {
		return b.logToDevServer(level, formatted)
	}
}

// Create a new App Engine-aware logger
func New() *Logger {
	log := new(Logger)

	// Backend that is appengine-aware
	backend := new(AppengineBackend)
	log.appengineBackend = backend

	// Log formatters, color for dev, plain for production
	plainFormatter := logging.MustStringFormatter("%{shortfile} %{longfunc} %{message}")
	colorFormatter := logging.MustStringFormatter("%{color}%{level:.5s} %{shortfile} %{longfunc} %{color:reset}%{message}")

	// Use plain formatter for production logging, color for dev server
	defaultBackend := logging.NewBackendFormatter(backend, plainFormatter)
	if appengine.IsDevAppServer() {
		defaultBackend = logging.NewBackendFormatter(backend, colorFormatter)
	}

	multiBackend := logging.SetBackend(defaultBackend)
	log.SetBackend(multiBackend)
	log.SetVerbose(appengine.IsDevAppServer())
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
		std.Debug("%s", args...)
	case string:
		std.Debug(v, args...)
	}
}

func Info(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		std.Info("%s", args...)
	case string:
		std.Info(v, args...)
	}
}

func Warn(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		std.Warning("%s", args...)
	case string:
		std.Warning(v, args...)
	}
}

func Error(formatOrError interface{}, args ...interface{}) {
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		std.Error("%s", args...)
	case string:
		std.Error(v, args...)
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

func Dump(args ...interface{}) {
	spew.Config.Indent = "  "
	dump := spew.Sdump(args...)
	std.Dump("\n%s", dump)
}

func Escape(s string) string {
	return strings.Replace(s, "%", "%%", -1)
}
