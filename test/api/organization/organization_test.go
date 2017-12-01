package test

import (
	"net/http"
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/models/wallet"
	"hanzo.io/util/gincontext"
	//"hanzo.io/util/log"
	"hanzo.io/util/permission"
	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/ginclient"

	. "hanzo.io/util/test/ginkgo"

	organizationApi "hanzo.io/api/organization"
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
	organizationApi.Route(cl.Router, adminRequired)

	// Create organization for tests, accessToken
	accessToken = org.AddToken("test-published-key", permission.Admin)
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

	usr3 := user.New(db)
	usr3.FirstName = "Z"
	usr3.LastName = "T"
	usr3.Email = "zack@taylor.edu"
	usr3.Enabled = false
	usr3.MustPut()

	usr4 := user.New(db)
	usr4.FirstName = "Z"
	usr4.LastName = "T"
	usr4.Email = "dev@hanzo.ai"
	usr4.SetPassword("blackisthenewred")
	usr4.Enabled = true
	usr4.MustPut()
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

type createRes struct {
	wallet.WalletHolder
}

type loginRes struct {
	Token string `json:"token"`
}

var _ = Describe("organization", func() {
	Context("Create", func() {
		It("Should retrieve wallet", func() {
			res := createRes{}

			cl.Get("/c/organization/"+org.Id()+"/wallet", &res)
		})
	})
})
