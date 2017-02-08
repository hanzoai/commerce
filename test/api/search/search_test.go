package search

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/context"

	"hanzo.io/datastore"
	"hanzo.io/models/app"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/log"
	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/ginclient"

	. "hanzo.io/util/test/ginkgo"

	searchApi "hanzo.io/api/search"
	token "hanzo.io/models/token2"
)

func Test(t *testing.T) {
	Setup("api/search", t)
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
	// adminRequired := middleware.TokenRequired(permission.Admin)

	// Create a new app engine context
	ctx, inst, _ = ae.NewContext()

	// Create mock gin context that we can use with fixtures
	c := gincontext.New(ctx)

	// Run fixtures
	u = fixtures.User(c).(*user.User)
	org = fixtures.Organization(c).(*organization.Organization)

	// Setup client and add routes for search API tests.
	client = ginclient.New(ctx)
	searchApi.Route(client.Router)

	// Create organization for tests, apiKey
	// Save namespaced db
	var err error
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
	Expect(err).NotTo(HaveOccurred())

	currentToken, _ = apiKey.IssueAccessToken(usr.Id(), ap.SecretKey)

	// currentToken = apiKey.String

	// Set authorization header for subsequent requests
	client.Setup(func(r *http.Request) {
		r.Header.Set("Authorization", currentToken)
	})

})

// Tear-down appengine context
var _ = AfterSuite(func() {
	inst.Close()
})

var _ = Describe("Search API", func() {
	Context("Search specific kind", func() {
		It("should return users matching the query", func() {
			time.Sleep(10)
			// query := "Email:dev@hanzo.ai"
			// limit := 30
			// offset := 0
			// encodedUrl := fmt.Sprintf("/search/user?query=%v&limit=%v&offset=%v", url.QueryEscape(query), limit, offset)
			encodedUrl := fmt.Sprintf("/search/user")

			w := client.Get(encodedUrl)
			log.Warn("%s", w.Body.String())

			Expect(w.Code).To(Equal(200))
		})
	})
})
