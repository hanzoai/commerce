package test

import (
	"testing"

	"golang.org/x/net/context"

	"hanzo.io/util/test/ae"
	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("util/task/integration", t)
}

var (
	ctx  context.Context
	inst ae.Instance
)

// Setup appengine context
var _ = BeforeSuite(func() {
	ctx, inst, _ = ae.NewContext(ae.Options{
		Modules:    []string{"default"},
		TaskQueues: []string{"default"},
	})
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	inst.Close()
})
