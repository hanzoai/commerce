package integration

import (
	"net/http"
	"testing"

	"golang.org/x/net/context"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/app"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
	"hanzo.io/util/permission"
	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/ginclient"

	. "hanzo.io/util/test/ginkgo"

	accountApi "hanzo.io/api/account"
	token "hanzo.io/models/token2"
)

func Test(t *testing.T) {
	Setup("api/account/integration", t)
}

var (
	ap           *app.App
	apiKey       *token.Token
	client       *ginclient.Client
	ctx          context.Context
	currentToken string
	db           *datastore.Datastore
	inst         ae.Instance
	org          *organization.Organization
	u            *user.User
	usr          *user.User
	usr2         *user.User
)

// Setup appengine context
var _ = BeforeSuite(func() {
	adminRequired := middleware.TokenRequired(permission.Admin)

	// Create a new app engine context
	var err error
	ctx, inst, err = ae.NewContext()
	Expect(err).NotTo(HaveOccurred())

	// Create mock gin context that we can use with fixtures
	c := gincontext.New(ctx)

	// Run fixtures
	u = fixtures.User(c).(*user.User)
	org = fixtures.Organization(c).(*organization.Organization)

	// Setup client and add routes for account API tests.
	client = ginclient.New(ctx)
	accountApi.Route(client.Router, adminRequired)

	// Create organization for tests, apiKey
	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))
	usr = user.New(db)
	usr.Email = "dev@hanzo.ai"
	usr.SetPassword("Z0rd0N")
	usr.Enabled = true
	usr.MustPut()

	usr2 = user.New(db)
	usr2.Email = "dev@hanzo.ai"
	usr2.SetPassword("ilikedragons")
	usr2.Enabled = false
	usr2.MustPut()

	ap = app.New(db)
	ap.GetById(organization.DefaultAppName)

	apiKey, _, err = ap.GetApiKeyByName(app.TestPublishedKey)
	// log.Warn("%v", apiKey)
	Expect(err).NotTo(HaveOccurred())

	currentToken = apiKey.String

	// Set authorization header for subsequent requests
	client.Setup(func(r *http.Request) {
		r.Header.Set("Authorization", currentToken)
	})
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	inst.Close()
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

			w := client.PostRawJSON("/account/login", req)
			json.DecodeBuffer(w.Body, &res)

			log.Debug("%#v %#v", req, w.Body)

			Expect(w.Code).To(Equal(200))
			// TODO: should deconstruct token and test if the user id is in it
			Expect(res.Token).ToNot(Equal(""))
		})

		It("Should disallow login with disabled account", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "ilikedragon"
			}`
			res := loginRes{}

			w := client.PostRawJSON("/account/login", req)
			json.DecodeBuffer(w.Body, &res)

			log.Debug("%#v %#v", req, res)

			Expect(w.Code).To(Equal(401))
			Expect(res.Token).To(Equal(""))
		})

		It("Should disallow login with wrong password", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "z3d"
			}`
			res := loginRes{}

			w := client.PostRawJSON("/account/login", req)
			json.DecodeBuffer(w.Body, &res)

			log.Debug("%#v %#v", req, res)

			Expect(w.Code).To(Equal(401))
			Expect(res.Token).To(Equal(""))
		})

		It("Should disallow login with wrong email", func() {
			req := `{
				"email": "billy@blue.co.uk",
				"password": "bloo"
			}`
			res := loginRes{}

			w := client.PostRawJSON("/account/login", req)
			json.DecodeBuffer(w.Body, &res)

			log.Debug("%#v %#v", req, res)

			Expect(w.Code).To(Equal(401))
			Expect(res.Token).To(Equal(""))
		})

		It("Should disallow login with customer token", func() {
			currentToken, _ = apiKey.IssueAccessToken(usr.Id(), ap.SecretKey)
			req := `{
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N"
			}`

			w := client.PostRawJSON("/account/login", req)
			currentToken = apiKey.String

			res := loginRes{}
			json.DecodeBuffer(w.Body, &res)

			log.Debug("%#v %#v", req, res)

			Expect(w.Code).To(Equal(401))
		})
	})

	Context("Get Information", func() {
		It("Should allow access if key is valid", func() {
			var err error
			currentToken, err = apiKey.IssueAccessToken(usr.Id(), ap.SecretKey)
			Expect(err).NotTo(HaveOccurred())

			w := client.Get("/account")
			currentToken = apiKey.String

			res := *user.New(db)
			json.DecodeBuffer(w.Body, &res)

			Expect(w.Code).To(Equal(200))
			Expect(res.Id()).To(Equal(usr.Id()))
		})

		It("Should deny access if key is not an access token", func() {
			w := client.Get("/account")

			res := *user.New(db)
			json.DecodeBuffer(w.Body, &res)

			Expect(w.Code).To(Equal(401))
		})

		It("Should deny access if user does not exist", func() {
			var err error
			currentToken, err = apiKey.IssueAccessToken(user.New(db).Id(), ap.SecretKey)
			Expect(err).NotTo(HaveOccurred())

			w := client.Get("/account")

			res := *user.New(db)
			json.DecodeBuffer(w.Body, &res)
			currentToken = apiKey.String

			Expect(w.Code).To(Equal(401))
		})
	})

	Context("Token Revokation", func() {
		It("Should deny login if apiKey is revoked", func() {
			apiKey.Revoke()

			// log.Warn(apiKey)

			req := `{
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N"
			}`
			res := loginRes{}

			w := client.PostRawJSON("/account/login", req)
			json.DecodeBuffer(w.Body, &res)

			log.Debug("%#v %#v", req, res)

			Expect(w.Code).To(Equal(401))

			ap.ResetDefaultKeys()
			apiKey, _, _ = ap.GetApiKeyByName(app.TestPublishedKey)
			currentToken = apiKey.String
		})

		It("Should deny access for access token if apiKey is revoked", func() {
			var err error
			currentToken, err = apiKey.IssueAccessToken(user.New(db).Id(), ap.SecretKey)
			Expect(err).NotTo(HaveOccurred())

			apiKey.Revoke()

			w := client.Get("/account")

			res := *user.New(db)
			json.DecodeBuffer(w.Body, &res)

			Expect(w.Code).To(Equal(401))

			ap.ResetDefaultKeys()
			apiKey, _, _ = ap.GetApiKeyByName(app.TestPublishedKey)
			currentToken = apiKey.String
		})
	})
})
