package test

import (
	"net/http"
	"strconv"
	"testing"

	billingApi "github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/fixtures"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
	"github.com/hanzoai/commerce/util/test/zipclient"
	"github.com/hanzoai/commerce/util/zipctx"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

// The suite drives the FULL billing surface, which includes money-MINT routes
// (credit-grants, deposit) reachable only by a SuperAdmin: an IAM principal whose
// home org is the reserved "admin" org. It presents the identity the boundary
// mints for the platform's own application (X-User-Owner: admin), the same
// principal api/billing's platformApp stamps, and pins X-Org-Id to the fixture
// org ("suchtees") so every spec stays in one namespace.
func Test(t *testing.T) {
	Setup("api/billing", t)
}

var (
	ctx         ae.Context
	db          *datastore.Datastore
	org         *organization.Organization
	cl          *zipclient.Client
	accessToken string
)

// Setup test context
var _ = BeforeSuite(func() {
	// Create new test context
	ctx = ae.NewContext()

	// Synthetic request context for fixtures
	c := zipctx.New(ctx)

	// Run default fixtures to setup organization
	org = fixtures.Organization(c).(*organization.Organization)
	tok, _ := org.GetTokenByName("test-secret-key")
	org.MustUpdate()

	accessToken = tok.String

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))

	// Create client
	cl = zipclient.New(ctx)

	// A per-org access token carries only org-level Admin and is (correctly) 403
	// on the mint routes: an org owner must never self-mint spendable balance.
	perms := strconv.FormatInt(int64(permission.Admin|permission.Live), 10)
	cl.Defaults(func(r *http.Request) {
		r.Header.Set("X-User-Id", "app_platform")
		r.Header.Set("X-User-Owner", "admin")
		r.Header.Set("X-Org-Id", org.Name)
		r.Header.Set("X-User-Permissions", perms)
	})

	cl.IgnoreErrors(true)

	// Validate the identity headers the way the boundary does in production,
	// then add the billing API routes behind it.
	cl.Router.Use(iammiddleware.IAMTokenRequired())
	billingApi.Route(cl.Router)
})

// Tear-down test context
var _ = AfterSuite(func() {
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
