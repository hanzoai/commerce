package test

import (
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/fixtures"
	"github.com/hanzoai/commerce/util/gincontext"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/mixin", t)
}

var (
	ctx  ae.Context
	db   *datastore.Datastore
	bcDb *datastore.Datastore
)

// Setup test context
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	c := gincontext.New(ctx)

	// Create the suchtees org and namespace for realism
	fixtures.Organization(c)

	// Create dbs for the two namespaces
	nsCtx := nscontext.WithNamespace(ctx, "suchtees")
	db = datastore.New(nsCtx)

	nsCtx = nscontext.WithNamespace(ctx, "_blockchains")
	bcDb = datastore.New(nsCtx)
})

// Tear-down test context
var _ = AfterSuite(func() {
	ctx.Close()
})
