package test

import (
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/api"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/fixtures"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/test/ae"
	"github.com/hanzoai/commerce/util/test/zipclient"
	"github.com/hanzoai/commerce/util/zipctx"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("thirdparty/shipstation", t)
}

var (
	ctx  ae.Context
	db   *datastore.Datastore
	org  *organization.Organization
	cl   *zipclient.Client
	bacl *zipclient.Client
)

// Setup test context
var _ = BeforeSuite(func() {
	// Create new test context
	ctx = ae.NewContext()

	// Mock gin context that we can use with fixtures
	c := zipctx.New(ctx)

	// Run default fixtures to setup organization and default store
	org = fixtures.Organization(c).(*organization.Organization)
	accessToken, _ := org.GetTokenByName("test-secret-key")
	org.MustUpdate()

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))

	// Client for API calls
	cl = zipclient.New(ctx)
	cl.Defaults(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken.String)
	})

	// Client for basic auth calls
	bacl = zipclient.New(ctx)
	bacl.Defaults(func(r *http.Request) {
		r.SetBasicAuth("dev@hanzo.ai", "suchtees")
	})

	// Add API routes to clients
	api.Route(cl.Router)
	api.Route(bacl.Router)
})

// Tear-down test context
var _ = AfterSuite(func() {
	ctx.Close()
})
