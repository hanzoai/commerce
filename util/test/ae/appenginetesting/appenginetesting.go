package appenginetesting

import (
	"path/filepath"

	. "github.com/onsi/ginkgo"
	"github.com/zeekay/appenginetesting"

	"crowdstart.io/util/test/ae/options"
)

// Add a module to options
func addModule(opts *appenginetesting.Options, moduleName string) {
	var modulePath string

	if moduleName == "default" {
		modulePath = filepath.Join("../../../../config/test/app.yaml")
	} else {
		modulePath = filepath.Join("../../../../config", moduleName, "/app.dev.yaml")
	}

	config := appenginetesting.ModuleConfig{
		Name: moduleName,
		Path: modulePath,
	}

	opts.Modules = append(opts.Modules, config)
}

// Create a new *appenginetesting.Context
func New(opts options.Options) (*appenginetesting.Context, error) {
	// Convert options.Options into *appenginetesting.Options
	_opts := &appenginetesting.Options{
		AppId:      opts.AppId,
		Debug:      appenginetesting.LogWarning,
		Testing:    GinkgoT(),
		TaskQueues: opts.TaskQueues,
	}

	// Add modules
	_opts.Modules = make([]appenginetesting.ModuleConfig, 0)

	for _, moduleName := range opts.Modules {
		addModule(_opts, moduleName)
	}

	return appenginetesting.NewContext(_opts)
}
