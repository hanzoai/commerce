package main

import (
	"google.golang.org/appengine"

	"github.com/hanzoai/commerce/util/default_"

	// Imported for side-effect, ensures tasks are registered
	_ "github.com/hanzoai/commerce/test-integration/datastore/tasks"
	_ "github.com/hanzoai/commerce/test-integration/util/task/tasks"
)

func init() {
	default_.Init()
}

func main() {
	appengine.Main()
}
