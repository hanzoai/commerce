package ae

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/phayes/freeport"
	"google.golang.org/appengine"
	"google.golang.org/appengine/aetest"

	"hanzo.io/log"
	"hanzo.io/util/retry"
)

func NewContext(args ...Options) Context {
	var (
		opts Options
		err  error
	)

	// Parse options
	switch len(args) {
	case 0:
		opts = defaults()
	case 1:
		opts = defaults(args[0])
	default:
		log.Panic("At most one ae.Options argument may be supplied.")
	}

	ports := []string{
		"DEV_APP_SERVER_ADMIN_PORT",
		"DEV_APP_SERVER_API_PORT",
		"DEV_APP_SERVER_PORT",
	}

	aetest.PrepareDevAppserver = func() error {
		// Loop over services and find available ports
		for _, service := range ports {
			// Get free port
			port, err := freeport.GetFreePort()
			if err != nil {
				return err
			}

			// Convert port into a string and update environment for
			// dev_appserver wrapper
			s := strconv.Itoa(port)
			os.Setenv(service, s)
		}

		// Derive project path from GOPATH
		projectDir := filepath.Join(os.Getenv("GOPATH"), "../..")

		// Ensure our wrapper is used
		os.Setenv("APPENGINE_DEV_APPSERVER", projectDir+"/scripts/dev_appserver.py")
		return nil
	}

	// Create new dev server instance
	var inst aetest.Instance

	err = retry.Retry(3, func() error {
		inst, err = aetest.NewInstance(&aetest.Options{
			AppID: opts.AppID,
			StronglyConsistentDatastore: opts.StronglyConsistentDatastore,
		})
		return err
	})

	if err != nil {
		log.Panic("Failed to create instance: %v", err)
	}

	// Create new request
	req, err := inst.NewRequest("GET", "/", nil)
	if err != nil {
		return nil
	}

	// Create new appengine context
	ctx := appengine.NewContext(req)

	// Return context lookalike with instance embedded
	return &context{ctx, inst}
}
