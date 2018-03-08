package env

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"syscall"

	"github.com/zeekay/aetest"

	"golang.org/x/net/context"
	"google.golang.org/appengine"

	"hanzo.io/util/log"
	"hanzo.io/util/test/ae/options"

	. "github.com/onsi/ginkgo"
)

// Trim out extraneous noise from logs
var logTrimRegexp = regexp.MustCompile(`  \d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2},\d{3}`)

// httpClient is used to communicate with the helper child process's
// webserver.  We can't use http.DefaultClient anymore, as it's now
// blacklisted in App Engine 1.6.1 due to people misusing it in blog
// posts and such.  (but this is one of the rare valid uses of not
// using urlfetch)
var httpClient = &http.Client{Transport: &http.Transport{Proxy: http.ProxyFromEnvironment}}

// Using -loglevel on the command line temporarily overrides the options in NewContext
var overrideLogLevel string

func init() {
	// TODO: Verify this works?
	// Check for override loglevel
	for i, a := range os.Args {
		if a == "-loglevel" {
			overrideLogLevel = os.Args[i+1]
		}
	}
}

var projectDir string

// Add a module to options
func addModule(ctx *FancyContext, moduleName string) {
	var modulePath string

	// Get absolute path to project root
	if projectDir == "" {
		_, filename, _, _ := runtime.Caller(1)
		projectDir = filepath.Join(filepath.Dir(filename), "../../../../")
	}

	// Default module is treated a bit differently, it's in config/ along with
	// relevant configuration.
	if moduleName == "default" {
		modulePath = filepath.Join(projectDir, "config/test/app.yaml")
	} else {
		modulePath = filepath.Join(projectDir, moduleName, "/app.dev.yaml")
	}

	// Create configuration for this module
	config := ModuleConfig{
		Name: moduleName,
		Path: modulePath,
	}

	// Append to modules
	ctx.Modules = append(ctx.Modules, config)
}

// Create a new *appenginetesting.Context
func New(opts options.Options) (context.Context, *Instance, error) {
	opts.SetDefaults()

	// Enable LogChild to be toggled on
	var level LogLevel
	if opts.LogChild {
		level = LogChild
	} else {
		level = LogInfo
	}

	req, _ := http.NewRequest("GET", "/", nil)
	ctx := &FancyContext{
		AppId:   opts.AppId,
		Queues:  opts.TaskQueues,
		Req:     req,
		Testing: GinkgoT(),
		Debug:   level,
		Modules: make([]ModuleConfig, 0),
	}
	for _, module := range opts.Modules {
		addModule(ctx, module)
	}
	log.Warn("Module %v", ctx.Modules)

	if opts.AppId == "" && len(ctx.Modules) > 0 {
		return nil, nil, fmt.Errorf("Options.AppId required if using Modules")
	}

	for _, mod := range ctx.Modules {
		if !fileExists(mod.Path) {
			return nil, nil, fmt.Errorf("File %s not found for module %s!", mod.Path, mod.Name)
		}
	}

	if err := StartChild(ctx); err != nil {
		return nil, nil, err
	}

	// in the hopes that the test program runs long, clean up non-closed Contexts
	runtime.SetFinalizer(ctx, func(deadContext *FancyContext) {
		Close(deadContext)
	})

	_opts := &aetest.Options{
		AppID: opts.AppId,
		StronglyConsistentDatastore: true,
	}
	inst, err := aetest.NewInstance(_opts)
	if err != nil {
		log.Fatal(err)
	}

	req, err = inst.NewRequest("GET", "/", nil)
	if err != nil {
		log.Fatal(err)
	}

	return appengine.WithAPICallFunc(appengine.WithContext(ctx, req), Call), &Instance{ctx, inst}, nil
}

func Close(c context.Context) error {
	ctx, ok := c.(*FancyContext)
	if ctx == nil || ctx.Child == nil || !ok {
		return errors.New("Not a FancyContext")
	}
	defer func() {
		os.RemoveAll(ctx.FakeAppDir)
	}()
	if p := ctx.Child.Process; p != nil {
		p.Signal(syscall.SIGTERM)
		if _, err := p.Wait(); err != nil {
			log.Fatal("Error closing devappserver - %v", err)
		}
	}
	ctx.Child = nil
	return nil
}

type ComponentURL struct {
	Name  string
	Regex *regexp.Regexp
	URL   string
}
