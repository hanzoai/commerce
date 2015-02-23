package crowdstart

import (
	"crowdstart.io/_default"

	// Imported for side-effect, ensures tasks are registered
	_ "crowdstart.io/test/datastore/integration/tasks"
	_ "crowdstart.io/test/util/task/integration/tasks"
)

func init() {
	_default.Init()
}
