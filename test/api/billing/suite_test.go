package test

import (
	"net/http"
	"os"
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

// serviceToken is the internal service-to-service secret (cloud-api → commerce).
// This suite drives the FULL billing surface, which since C1 includes money-MINT
// routes (credit-grants, deposit) that are PlatformOnly — reachable ONLY by the
// verified service token or a platform global admin, never a plain org-admin
// access token. The suite therefore authenticates as that service principal (the
// real production caller for these routes: cloud-api). It still resolves the SAME
// fixture org ("suchtees") via X-Org-Id, so every non-mint spec is unaffected.
const serviceToken = "test-commerce-service-token-billing-suite"

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

	// Authenticate the suite as the internal service token (the production caller
	// for the money-mint billing routes since C1). TokenRequired verifies the
	// bearer against COMMERCE_SERVICE_TOKEN, marks the request a verified service
	// caller (→ MayMintMoney true, PlatformOnly passes), and resolves the org from
	// X-Org-Id — pinned to the fixture org so all specs stay in one namespace.
	os.Setenv("COMMERCE_SERVICE_TOKEN", serviceToken)

	// Create client
	cl = ginclient.New(ctx)

	// Set authorization header
	cl.Defaults(func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+serviceToken)
		r.Header.Set("X-Org-Id", org.Name)
	})

	cl.IgnoreErrors(true)

	// Add billing API routes to client
	billingApi.Route(cl.Router)
})

// Tear-down test context
var _ = AfterSuite(func() {
	os.Unsetenv("COMMERCE_SERVICE_TOKEN")
	ctx.Close()
})
