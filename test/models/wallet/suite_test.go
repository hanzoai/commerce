package test

import (
	"appengine"
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/models/fixtures"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/mixin", t)
}

var (
	ctx  ae.Context
	db   *datastore.Datastore
	bcDb *datastore.Datastore
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	c := gincontext.New(ctx)

	// We need to create the blockchain namespace
	fixtures.BlockchainNamespace(c)

	// Create the suchtees org and namespace for realism
	fixtures.Organization(c)

	// Create dbs for the two namespaces
	nsCtx, _ := appengine.Namespace(ctx, "suchtees")
	db = datastore.New(nsCtx)

	nsCtx, _ = appengine.Namespace(ctx, "_blockchains")
	bcDb = datastore.New(nsCtx)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})
