package integration

import (
	"net/http"
	"testing"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/fixtures"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"
	"crowdstart.com/util/gincontext"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/test/ae"
	"crowdstart.com/util/test/ginclient"

	. "crowdstart.com/util/test/ginkgo"

	accountApi "crowdstart.com/api/account"
)

func Test(t *testing.T) {
	Setup("api/account", t)
}

var (
	ctx         ae.Context
	cl          *ginclient.Client
	accessToken string
	db          *datastore.Datastore
	org         *organization.Organization
	u           *user.User
)

// Setup appengine context
var _ = BeforeSuite(func() {
	adminRequired := middleware.TokenRequired(permission.Admin)

	// Create a new app engine context
	ctx = ae.NewContext()

	// Create mock gin context that we can use with fixtures
	c := gincontext.New(ctx)

	// Run fixtures
	u = fixtures.User(c).(*user.User)
	org = fixtures.Organization(c).(*organization.Organization)

	// Setup client and add routes for account API tests.
	cl = ginclient.New(ctx)
	accountApi.Route(cl.Router, adminRequired)

	// Create organization for tests, accessToken
	accessToken = org.AddToken("test-published-key", permission.Published)
	err := org.Put()
	Expect(err).NotTo(HaveOccurred())

	// Set authorization header for subsequent requests
	cl.Defaults(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken)
	})

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))
	usr := user.New(db)
	usr.Email = "dev@hanzo.ai"
	usr.SetPassword("Z0rd0N")
	usr.Enabled = true
	usr.MustPut()

	usr2 := user.New(db)
	usr2.Email = "dev@hanzo.ai"
	usr2.SetPassword("ilikedragons")
	usr2.Enabled = false
	usr2.MustPut()
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

type loginRes struct {
	Token string `json:"token"`
}

var _ = Describe("account", func() {
	Context("Login", func() {
		It("Should allow login with proper credentials", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N"
			}`
			res := loginRes{}

			cl.Post("/account/login", req, res)
			// TODO: should deconstruct token and test if the user id is in it
			Expect(res.Token).ToNot(Equal(""))
		})

		It("Should disallow login with disabled account", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "ilikedragon"
			}`
			res := loginRes{}

			cl.Post("/account/login", req, res, 401)
			Expect(res.Token).To(Equal(""))
		})

		It("Should disallow login with wrong password", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "z3d"
			}`
			res := loginRes{}

			cl.Post("/account/login", req, res, 401)
			Expect(res.Token).To(Equal(""))
		})

		It("Should disallow login with wrong email", func() {
			req := `{
				"email": "billy@blue.co.uk",
				"password": "bloo"
			}`
			res := loginRes{}

			cl.Post("/account/login", req, res, 401)
			Expect(res.Token).To(Equal(""))
		})
	})
})
