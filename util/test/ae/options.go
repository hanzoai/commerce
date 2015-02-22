package ae

import (
	"crowdstart.io/util/test/ae/options"
)

// Aliased for ease of use.
type Options options.Options

func (c *Options) SetDefaults() {
	if c.AppId == "" {
		c.AppId = "crowdstart-io"
	}

	if c.TaskQueues == nil {
		c.TaskQueues = make([]string, 0)
	}

	if c.Modules == nil {
		c.Modules = make([]string, 0)
	}

	if len(c.Modules) == 0 {
		c.Modules = append(c.Modules, "default")
	}
}
