package test

import (
	"testing"

	"golang.org/x/net/context"

	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("util/val", t)
}

var (
	ctx  context.Context
	inst ae.Instance
)

var _ = BeforeSuite(func() {
	// Create a new app engine context
	ctx, inst, _ = ae.NewContext()
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	inst.Close()
})
