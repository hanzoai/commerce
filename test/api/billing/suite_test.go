package test

import (
	"net/http"
	"os"
	"testing"

	billingApi "github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/fixtures"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/gincontext"
	"github.com/hanzoai/commerce/util/test/ae"
	"github.com/hanzoai/commerce/util/test/ginclient"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

// serviceToken is the internal service-to-service secret this suite presents.
// The billing endpoints are the cloud-api → commerce surface, and the money-MINT
// routes (credit-grants, deposit, refund, grant-starter, allotment) are gated to
// the internal service token / platform global admin ONLY (middleware.PlatformOnly).
const serviceToken = "test-service-token"

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

	// Authenticate the way cloud-api actually calls commerce billing: the
	// internal service token (Bearer COMMERCE_SERVICE_TOKEN) + the target org
	// (X-Org-Id). TokenRequired's service-token branch verifies the bearer,
	// resolves the fixture org, and stamps the service-token marker that
	// middleware.PlatformOnly requires for the money-MINT routes. A per-org
	// access token carries only org-level Admin and is (correctly) 403 on those
	// routes — an org owner must never be able to self-mint spendable balance.
	os.Setenv("COMMERCE_SERVICE_TOKEN", serviceToken)
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

// seedCharge writes a Withdraw (charge) to the fixture org's ledger and returns
// its id — the thing a refund reverses. It mirrors the money model the Refund
// handler validates after the H1 hardening (an original that exists, is a
// Withdraw, and is owned by the subject); see api/billing/refund_h1_test.go's
// seedCharge. A refund of a Deposit is (correctly) rejected — refunding a credit
// would double it — so the refund test must reverse a real charge, not a deposit.
func seedCharge(subject string, cents int64) string {
	tr := transaction.New(db)
	tr.Type = transaction.Withdraw
	tr.DestinationId = subject
	tr.DestinationKind = "iam-user"
	tr.Currency = currency.USD
	tr.Amount = currency.Cents(cents)
	tr.MustCreate()
	return tr.Id()
}
