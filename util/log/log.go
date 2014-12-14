package log

import (
	"log"
	"strings"

	"appengine"

	"github.com/gin-gonic/gin"
	"github.com/zeekay/go-logging"
	// "github.com/davecgh/go-spew/spew"

	"crowdstart.io/config"
	"crowdstart.io/thirdparty/sentry"
)

// Custom logger
type Logger struct {
	logging.Logger
	appengineBackend *AppengineBackend
}

// Process args, setting app engine context if passed one.
func (l *Logger) setContext(args ...interface{}) []interface{} {
	if len(args) == 0 {
		return args
	}

	// Appengine context is last argument
	ctx := args[len(args)-1]

	switch ctx := ctx.(type) {
	case *gin.Context:
		appengineContext := ctx.MustGet("appengine").(appengine.Context)
		l.appengineBackend.context = appengineContext
		l.appengineBackend.requestURI = ctx.Request.RequestURI
		args = args[:len(args)-1]
	case appengine.Context:
		l.appengineBackend.context = ctx
		args = args[:len(args)-1]
	default:
		l.appengineBackend.context = nil
	}

	// Last/second to last argument MIGHT be an error
	if len(args) > 0 {
		if err, ok := args[len(args)-1].(error); ok {
			l.appengineBackend.error = err
		}
	}

	return args
}

// Custom logger backend that knows about AppEngine
type AppengineBackend struct {
	context    appengine.Context
	requestURI string
	error      error
}

func logToSentry(ctx appengine.Context, formatted, requestURI string, err error) {
	// Log to sentry asynchronously
	if config.SentryDSN != "" {
		if err != nil {
			exc := sentry.NewException(err)
			sentry.CaptureException.Call(ctx, requestURI, exc)
		} else {
			exc := sentry.NewExceptionFromStack(formatted)
			sentry.CaptureException.Call(ctx, requestURI, exc)
		}
	}
}

func (b AppengineBackend) Log(level logging.Level, calldepth int, record *logging.Record) error {
	formatted := record.Formatted(calldepth + 2)

	if b.context != nil {
		switch level {
		case logging.WARNING:
			b.context.Warningf(formatted)
		case logging.ERROR:
			b.context.Errorf(formatted)
			// TODO: Clean up code base to make this feasible
			// logToSentry(b.context, formatted, b.requestURI, b.error)
		case logging.CRITICAL:
			b.context.Criticalf(formatted)
			logToSentry(b.context, formatted, b.requestURI, b.error)
		case logging.INFO:
			b.context.Infof(formatted)
		default:
			b.context.Debugf(formatted)
		}
	} else {
		// Hack to make INFO level less verbose
		if level == logging.INFO {
			parts := strings.Split(formatted, " ")
			parts = append([]string{"INFO"}, parts[3:]...)
			formatted = strings.Join(parts, " ")
		}
		log.Println(formatted)
	}

	return nil
}

func New() *Logger {
	log := new(Logger)

	// Backend that is appengine-aware
	backend := new(AppengineBackend)
	log.appengineBackend = backend

	// Log formatters, color for dev, plain for production
	plainFormatter := logging.MustStringFormatter("%{shortfile} %{longfunc} %{message}")
	colorFormatter := logging.MustStringFormatter("%{color}%{level:.5s} %{shortfile} %{longfunc} %{color:reset}%{message}")

	defaultBackend := logging.NewBackendFormatter(backend, plainFormatter)

	if appengine.IsDevAppServer() {
		defaultBackend = logging.NewBackendFormatter(backend, colorFormatter)
	}

	multiBackend := logging.SetBackend(defaultBackend)
	log.SetBackend(multiBackend)
	return log
}

var std = New()

func Dump(args ...interface{}) {
	// spew.Config.Indent = "  "
	// dump := spew.Sdump(args...)
	// std.Dump("\n%s", dump)
}

func Debug(formatOrError interface{}, args ...interface{}) {
	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		args = std.setContext(args...)
		std.Debug("%s", args...)
	case string:
		args = std.setContext(args...)
		std.Debug(v, args...)
	}
}

func Info(formatOrError interface{}, args ...interface{}) {
	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		args = std.setContext(args...)
		std.Info("%s", args...)
	case string:
		args = std.setContext(args...)
		std.Info(v, args...)
	}
}

func Warn(formatOrError interface{}, args ...interface{}) {
	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		args = std.setContext(args...)
		std.Warning("%s", args...)
	case string:
		args = std.setContext(args...)
		std.Warning(v, args...)
	}
}

func Error(formatOrError interface{}, args ...interface{}) {
	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		args = std.setContext(args...)
		std.Error("%s", args...)
	case string:
		args = std.setContext(args...)
		std.Error(v, args...)
	}
}

func Fatal(formatOrError interface{}, args ...interface{}) {
	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		args = std.setContext(args...)
		std.Fatalf("%s", args...)
	case string:
		args = std.setContext(args...)
		std.Fatalf(v, args...)
	}
}

func Panic(formatOrError interface{}, args ...interface{}) {
	switch v := formatOrError.(type) {
	case error:
		args = append([]interface{}{v}, args...)
		args = std.setContext(args...)
		std.Panicf("%s", args...)
	case string:
		args = std.setContext(args...)
		std.Panicf(v, args...)
	}
}
