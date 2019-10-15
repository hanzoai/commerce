package main

import (
	"google.golang.org/appengine"

	"hanzo.io/util/default_"

	// Imported for side-effect, ensures tasks are registered
	_ "hanzo.io/test-integration/datastore/tasks"
	_ "hanzo.io/test-integration/util/task/tasks"
)

func init() {
	default_.Init()
}

func main() {
	appengine.Main()
}
