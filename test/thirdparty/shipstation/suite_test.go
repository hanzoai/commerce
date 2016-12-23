package test

import (
	"net/http"
	"testing"

	"crowdstart.com/api"
	"crowdstart.com/datastore"
	"crowdstart.com/models/fixtures"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/gincontext"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/test/ae"
	"crowdstart.com/util/test/ginclient"

	. "crowdstart.com/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("thirdparty/shipstation", t)
}

var (
	ctx  ae.Context
	db   *datastore.Datastore
	org  *organization.Organization
	cl   *ginclient.Client
	bacl *ginclient.Client
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

	// Client for API calls
	cl = ginclient.New(ctx)
	cl.Defaults(func(r *http.Request) {
		r.SetBasicAuth("dev@hanzo.ai", "suchtees")
		r.Header.Set("Authorization", accessToken)
	})

	// Client for basic auth calls
	bacl = ginclient.New(ctx)
	bacl.Defaults(func(r *http.Request) {
		r.SetBasicAuth("dev@hanzo.ai", "suchtees")
	})

	// Add API routes to clients
	api.Route(cl.Router)
	api.Route(bacl.Router)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})
