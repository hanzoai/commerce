package log

import (
	"appengine"
	"log"

	"github.com/davecgh/go-spew/spew"
	"github.com/gin-gonic/gin"
	"github.com/zeekay/go-logging"
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

	ctx := args[len(args)-1]

	switch ctx := ctx.(type) {
	case *gin.Context:
		c := ctx.MustGet("appengine").(appengine.Context)
		l.appengineBackend.context = c
		args = args[:len(args)-1]
	case appengine.Context:
		l.appengineBackend.context = ctx
		args = args[:len(args)-1]
	default:
		l.appengineBackend.context = nil
	}

	return args
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
		default:
			b.context.Debugf(formatted)
		}
	} else {
		log.Println(formatted)
	}

	return nil
}

func New() *Logger {
	log := new(Logger)

	// Set spew to output tab indented dumps
	spew.Config.Indent = "  "

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
	dump := spew.Sdump(args...)
	std.Dump("\n%s", dump)
}

func Debug(format string, args ...interface{}) {
	args = std.setContext(args...)
	std.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	args = std.setContext(args...)
	std.Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	args = std.setContext(args...)
	std.Warning(format, args...)
}

func Error(format string, args ...interface{}) {
	args = std.setContext(args...)
	std.Error(format, args...)
}

func Fatal(format string, args ...interface{}) {
	args = std.setContext(args...)
	std.Fatalf(format, args...)
}

func Panic(format string, args ...interface{}) {
	args = std.setContext(args...)
	std.Panicf(format, args...)
}
