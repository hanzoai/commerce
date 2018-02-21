package ae

// Aliased for simplicity
type Options struct {
	AppID                       string
	StronglyConsistentDatastore bool
	Modules                     []string
	Debug                       bool

	// Deprecated
	TaskQueues []string
	Noisy      bool
}

// Generate safe defaults
func defaults() Options {
	return Options{
		AppID: "None",
		StronglyConsistentDatastore: true,
		Modules:                     []string{"default"},
		Debug:                       false,
	}
}
