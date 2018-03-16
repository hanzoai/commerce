package ae

import (
	"os"
	"path/filepath"
	"sort"
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

	services := []string{
		"DEV_APPSERVER_ADMIN_PORT",
		"DEV_APPSERVER_API_PORT",
		"DEV_APPSERVER_PORT",
	}

	// Find available ports
	ports := make([]int, 3)
	ports[0], _ = freeport.GetFreePort()
	ports[1], _ = freeport.GetFreePort()
	ports[2], _ = freeport.GetFreePort()

	// Sort least to highest, each service port is incremented up from
	// api_port, so ensure it has the highest port number
	sort.Slice(ports, func(i, j int) bool { return ports[i] < ports[j] })

	// Derive project path from GOPATH
	projectDir := filepath.Join(os.Getenv("GOPATH"), "../..")

	aetest.PrepareDevAppserver = func() error {
		// Convert port into a string and update environment for dev_appserver
		// wrapper
		for i, service := range services {
			s := strconv.Itoa(ports[i])
			os.Setenv(service, s)
		}

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
		if err != nil {
			inst.Close()
		}
		return nil
	})

	if err != nil {
		log.Panic("Failed to create instance: %v", err)
	}

	// Create new request
	req, err := inst.NewRequest("GET", "/", nil)
	if err != nil {
		log.Panic("Failed to create request")
	}

	// Create new appengine context
	ctx := appengine.NewContext(req)

	// Return context lookalike with instance embedded
	return &context{ctx, inst}
}
