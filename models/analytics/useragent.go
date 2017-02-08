package analytics

type UserAgent struct {
	Browser struct {
		Name    string
		Version string
	}
	Engine struct {
		Name    string
		Version string
	}
	Os struct {
		Name    string
		Version string
	}
	Device struct {
		Model  string
		Type   string
		Vendor string
	}
	Cpu struct {
		Architecture string
	}
}
