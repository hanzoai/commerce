package test

import (
	"google.golang.org/appengine"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/fixtures"
	"github.com/hanzoai/commerce/util/gincontext"
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

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	c := gincontext.New(ctx)

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
