package crowdstart

import (
	"crowdstart.com/_default"

	// Imported for side-effect, ensures tasks are registered
	_ "crowdstart.com/test/datastore/integration/tasks"
	_ "crowdstart.com/test/util/task/integration/tasks"
)

func init() {
	_default.Init()
}
