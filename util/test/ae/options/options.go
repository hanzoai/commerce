package options

type Options struct {
	AppId                    string
	DisableStrongConsistency bool
	Modules                  []string
	TaskQueues               []string
}
