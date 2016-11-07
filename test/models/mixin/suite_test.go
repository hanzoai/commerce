package test

import (
	"testing"

	"crowdstart.com/datastore"
	"crowdstart.com/models/fixtures"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/gincontext"
	"crowdstart.com/util/test/ae"

	. "crowdstart.com/util/test/ginkgo"
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

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)

	org := fixtures.Organization(c).(*organization.Organization)
	org.MustUpdate()
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})
