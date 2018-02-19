package appenginetesting

import (
	"path/filepath"
	"runtime"

	"github.com/davidtai/appenginetesting"
	. "github.com/onsi/ginkgo"

	"hanzo.io/log"
	"hanzo.io/util/test/ae/options"
)

var projectDir string

// Add a module to options
func addModule(opts *appenginetesting.Options, moduleName string) {
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
	config := appenginetesting.ModuleConfig{
		Name: moduleName,
		Path: modulePath,
	}

	// Append to modules
	opts.Modules = append(opts.Modules, config)
}

// Create a new *appenginetesting.Context
func New(opts options.Options) (*appenginetesting.Context, error) {
	opts.SetDefaults()

	// Convert options.Options into *appenginetesting.Options
	aeopts := &appenginetesting.Options{
		AppId:      opts.AppId,
		Testing:    GinkgoT(),
		TaskQueues: opts.TaskQueues,
	}

	// Detect verbose
	if log.Verbose() {
		aeopts.Debug = appenginetesting.LogDebug
	} else {
		aeopts.Debug = appenginetesting.LogWarning
	}

	// Override and spam everything
	if opts.Noisy {
		aeopts.Debug = appenginetesting.LogChild
	}

	// Add modules
	aeopts.Modules = make([]appenginetesting.ModuleConfig, 0)

	for _, moduleName := range opts.Modules {
		addModule(aeopts, moduleName)
	}

	log.Debug("Creating new appenginetesting context: %#v", aeopts)

	return appenginetesting.NewContext(aeopts)
}
