package env

import (
	"net/http"
	"os/exec"
	"time"
)

//The interface returned by GinkgoT().  This covers most of the methods
//in the testing package's T.
type TestingT interface {
	Fail()
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	FailNow()
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Failed() bool
	Parallel()
	Skip(args ...interface{})
	Skipf(format string, args ...interface{})
	SkipNow()
	Skipped() bool
}

type LogLevel string

const (
	LogChild    LogLevel = "child" // LogChild logs all log levels plus what comes from the devappserver process
	LogDebug             = "debug"
	LogInfo              = "info"
	LogWarning           = "warning"
	LogError             = "error"
	LogCritical          = "critical"
)

type FancyContext struct {
	AppId      string
	Child      *exec.Cmd
	Debug      LogLevel       // send the output of the application to console
	FakeAppDir string         // temp dir for application files
	Modules    []ModuleConfig // list of the modules that should start up on each test
	Queues     []string       // list of queues to support
	Req        *http.Request
	Testing    TestingT
	TestingURL string // URL of "stub" module to send requests toModules    []ModuleConfig // list of the modules that should start up on each test
}

type ModuleConfig struct {
	Name string // name of the module in the yaml file
	Path string // can be relative to the current working directory and should include the yaml file
}

func (c FancyContext) Deadline() (time.Time, bool) {
	return time.Now().Add(5 * time.Second), false
}

func (c FancyContext) Done() <-chan struct{} {
	return make(chan struct{})
}

func (c FancyContext) Err() error {
	return nil
}

func (c FancyContext) Value(k interface{}) interface{} {
	switch v := k.(type) {
	case *string:
		switch *v {
		case "holds a string, being the full app ID":
			return "dev~" + c.AppId
			// case "holds the namespace string":
			// 	return ""
		}
	case string:
		switch v {
		case "req":
			return c.Req
		case "testingURL":
			return c.TestingURL
		}
	}
	return nil
}
