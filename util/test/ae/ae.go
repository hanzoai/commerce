package ae

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/aetest"

	"hanzo.io/log"
)

func Close() error {
	err := inst.Close()
	inst = nil
	return err
}

var inst aetest.Instance

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

	// Share instance across NewContext requests
	if inst == nil {
		// Create new dev server instance
		inst, err = aetest.NewInstance(&aetest.Options{
			AppID: opts.AppID,
			StronglyConsistentDatastore: opts.StronglyConsistentDatastore,
		})
		if err != nil {
			log.Panic("Failed to create instance: %v", err)
		}
	}

	// Create new request
	req, err := inst.NewRequest("GET", "/", nil)
	if err != nil {
		return nil
	}

	// Create new appengine context
	ctx := appengine.NewContext(req)

	// Return context lookalike with instance embedded
	return &context{ctx}
}
