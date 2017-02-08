package crowdstart

import (
	"hanzo.io/_default"

	// Imported for side-effect, ensures tasks are registered
	_ "hanzo.io/test/datastore/integration/tasks"
	_ "hanzo.io/test/util/task/integration/tasks"
)

func init() {
	_default.Init()
}
