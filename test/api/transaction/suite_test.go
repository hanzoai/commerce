package test

import (
	"net/http"
	"testing"

	"hanzo.io/api"
	"hanzo.io/datastore"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/organization"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/permission"
	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/ginclient"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("api/transaction", t)
}

var (
	ctx ae.Context
	db  *datastore.Datastore
	org *organization.Organization
	cl  *ginclient.Client
)

// Setup appengine context
var _ = BeforeSuite(func() {
	// Create new appengine context
	ctx = ae.NewContext()

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)

	// Run default fixtures to setup organization and default store
	org = fixtures.Organization(c).(*organization.Organization)
	accessToken := org.AddToken("test-published-key", permission.Admin)
	org.MustUpdate()

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))

	// Create client so we can make requests
	cl = ginclient.New(ctx)

	// Set authorization header for subsequent requests
	cl.Defaults(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken)
	})

	cl.IgnoreErrors(true)

	// Add API routes to client
	api.Route(cl.Router)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})
