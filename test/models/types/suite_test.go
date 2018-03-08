package test

import (
	"testing"

	"golang.org/x/net/context"

	"hanzo.io/datastore"
	"hanzo.io/util/test/ae"
	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/types", t)
}

var (
	ctx  context.Context
	inst ae.Instance
	db   *datastore.Datastore
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx, inst, _ = ae.NewContext()
	db = datastore.New(ctx)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	inst.Close()
})
