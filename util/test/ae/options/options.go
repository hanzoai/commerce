package options

type Options struct {
	AppId                       string
	Modules                     []string
	StronglyConsistentDatastore bool
	TaskQueues                  []string
}
