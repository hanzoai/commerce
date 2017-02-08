package test

import (
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/util/test/ae"
	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/mixin", t)
}

var (
	ctx ae.Context
	db  *datastore.Datastore
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})
