package test

import (
	"net/http"
	"testing"

	billingApi "github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/fixtures"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/gincontext"
	"github.com/hanzoai/commerce/util/test/ae"
	"github.com/hanzoai/commerce/util/test/ginclient"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("api/billing", t)
}

var (
	ctx         ae.Context
	db          *datastore.Datastore
	org         *organization.Organization
	cl          *ginclient.Client
	accessToken string
)

// Setup test context
var _ = BeforeSuite(func() {
	// Create new test context
	ctx = ae.NewContext()

	// Mock gin context for fixtures
	c := gincontext.New(ctx)

	// Run default fixtures to setup organization
	org = fixtures.Organization(c).(*organization.Organization)
	tok, _ := org.GetTokenByName("test-secret-key")
	org.MustUpdate()

	accessToken = tok.String

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))

	// Create client
	cl = ginclient.New(ctx)

	// Set authorization header
	cl.Defaults(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken)
	})

	cl.IgnoreErrors(true)

	// Add billing API routes to client
	billingApi.Route(cl.Router)
})

// Tear-down test context
var _ = AfterSuite(func() {
	ctx.Close()
})
