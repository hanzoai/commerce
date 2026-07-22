package log

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"sync"

	"github.com/op/go-logging"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/spew"
)

// Create a new logger
func New() *Logger {
	log := new(Logger)
	log.backend = NewBackend(nil)

	// Log formatters, color for dev, plain for production
	plainFormatter := MustStringFormatter("%{longfile} %{longfunc} %{message}")
	colorFormatter := MustStringFormatter("%{color}%{level:.5s} %{longfile} %{longfunc} %{color:reset}%{message}")

	// Use plain formatter for production logging, color for dev server
	backend := logging.NewBackendFormatter(log.backend, plainFormatter)
	if !config.IsProduction {
		backend = logging.NewBackendFormatter(backend, colorFormatter)
	}

	log.SetBackend(logging.SetBackend(backend))

	if config.IsDevelopment {
		log.SetVerbose(true)
	}

	return log
}

var std = New()

// logMu serializes every package-level log call. parseArgs stashes per-request
// state (backend.context, backend.requestURI, backend.error, verboseRequested)
// on the shared std singleton, and the go-logging Backend reads that state back
// during the emit — a window that races when two goroutines log concurrently
// (e.g. a request goroutine and a fire-and-forget `go engine.…` worker both
// calling log.Debug). The go-logging Backend interface can't receive per-call
// context, so the state must live on the shared backend; making each stash→emit
// atomic is the same approach the stdlib `log` package takes. The Backend's Log
// writes only to os.Stdout/os.Stderr and never re-enters these funcs, so the
// lock cannot self-deadlock. Guard the package entry points ONLY — never the
// *Logger methods (parseArgs/Verbose/…), which run while this lock is held.
var logMu sync.Mutex

func SetVerbose(verbose bool) {
	logMu.Lock()
	defer logMu.Unlock()
	std.SetVerbose(verbose)
}

func Verbose() bool {
	logMu.Lock()
	defer logMu.Unlock()
	return std.Verbose()
}

func Debug(formatOrError interface{}, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	args = std.parseArgs(args...)

	if !std.Verbose() {
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
	logMu.Lock()
	defer logMu.Unlock()
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		std.Infof(errAndStack(v))
	case string:
		std.Infof(v, args...)
	}
}

func Warn(formatOrError interface{}, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		std.Warningf(errAndStack(v))
	case string:
		std.Warningf(v, args...)
	}
}

func Error(formatOrError interface{}, args ...interface{}) error {
	logMu.Lock()
	defer logMu.Unlock()
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		std.Errorf(errAndStack(v))
		fmt.Println(v)
		return v
	case string:
		std.Errorf(v, args...)
		fmt.Println(v)
		return fmt.Errorf(v, args...)
	}
	return nil
}

func Fatal(formatOrError interface{}, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		std.Fatalf(errAndStack(v))
	case string:
		std.Fatalf(v, args...)
	}
}

func Panic(formatOrError interface{}, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	args = std.parseArgs(args...)

	switch v := formatOrError.(type) {
	case error:
		std.Panicf(errAndStack(v))
	case string:
		std.Panicf(v, args...)
	}
}

func Dump(formatOrObject interface{}, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	args = std.parseArgs(args...)

	if !std.Verbose() {
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
	logMu.Lock()
	defer logMu.Unlock()
	args = std.parseArgs(args...)

	if !std.Verbose() {
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

func Request(req *http.Request, args ...interface{}) error {
	logMu.Lock()
	defer logMu.Unlock()
	args = std.parseArgs(args...)

	if !std.Verbose() {
		return nil
	}

	dump, err := httputil.DumpRequest(req, true)
	if err != nil {
		std.Errorf("Failed to dump request: %v", err)
		return fmt.Errorf("Failed to dump request: %v", err)
	}
	std.Debug(string(dump))
	return nil
}

func RequestOut(req *http.Request, args ...interface{}) error {
	logMu.Lock()
	defer logMu.Unlock()
	args = std.parseArgs(args...)

	if !std.Verbose() {
		return nil
	}

	dump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		std.Errorf("Failed to dump request: %v", err)
		return fmt.Errorf("Failed to dump request: %v", err)
	}
	std.Debug(string(dump))
	return nil
}

func Response(res *http.Response, args ...interface{}) error {
	logMu.Lock()
	defer logMu.Unlock()
	args = std.parseArgs(args...)

	if !std.Verbose() {
		return nil
	}

	dump, err := httputil.DumpResponse(res, true)
	if err != nil {
		std.Errorf("Failed to dump request: %v", err)
		return fmt.Errorf("Failed to dump request: %v", err)
	}
	std.Debug(string(dump))
	return nil
}

func Stack(args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	args = std.parseArgs(args...)

	if len(args) == 0 {
		std.Debugf(stack(4))
		return
	}

	formatOrError := args[0]

	switch v := formatOrError.(type) {
	case error:
		std.Debugf(errAndStack(v))
	case string:
		msg := fmt.Sprintf(v, args[1:]...)
		std.Debugf(msg + stack(4))
	}
}
