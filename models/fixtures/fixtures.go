package fixtures

import (
	"appengine"
	"appengine/delay"

	"crowdstart.io/config"
)

var fixtures = make(map[string][]*delay.Function)

// Add new fixture
func addFixture(name string, fns ...*delay.Function) {
	// Create slice for fixture set
	if _, ok := fixtures[name]; !ok {
		fixtures[name] = make([]*delay.Function, 0)
	}

	// Append fixture
	fixtures[name] = append(fixtures[name], fns...)
}

// Install fixtures
var Install = delay.Func("install-fixture", func(c appengine.Context, name string) {
	fns := fixtures[name]
	for _, fn := range fns {
		fn.Call(c)
	}
})

// Define all fixtures
func init() {
	// user fixtures
	addFixture("users", testUsers)

	if !config.IsProduction {
		addFixture("users", skullyUser)
		addFixture("skully-campaign", skullyCampaign)
		addFixture("contributors", contributors)
	}

	addFixture("products", products)

	// Temp fix to update international user data
	// addFixture("international", products)

	// Add helper to install all fixtures
	for _, fns := range fixtures {
		addFixture("all", fns...)
	}
}
