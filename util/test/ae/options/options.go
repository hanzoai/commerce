package options

type Options struct {
	AppId                    string
	DisableStrongConsistency bool
	Modules                  []string
	TaskQueues               []string
	PreferAppengineTesting   bool
	Noisy                    bool
}

func (c *Options) SetDefaults() {
	if c.AppId == "" {
		c.AppId = "development"
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
