package main

import (
	"hanzo.io/util/default_"

	// Imported for side-effect, ensures tasks are registered
	_ "hanzo.io/test/datastore/integration/tasks"
	_ "hanzo.io/test/util/task/integration/tasks"
)

func main() {
	default_.Init()
}
