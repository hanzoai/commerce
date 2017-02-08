package test

import (
	"net/http"
	"testing"

	"golang.org/x/net/context"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/app"
	"hanzo.io/models/organization"
	"hanzo.io/models/token2"
	"hanzo.io/test/fixtures/user"
	"hanzo.io/util/bit"
	"hanzo.io/util/rest"
	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/ginclient"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("util/rest", t)
}

var (
	ctx  context.Context
	inst ae.Instance
	tok1 string
	tok2 string
)

const (
	Perm1 bit.Mask = 1 << iota // 1 << 0 which is 00000001
	Perm2
	Perm3
)

// Setup appengine context
var _ = BeforeSuite(func() {
	ctx, inst, _ = ae.NewContext()

	// Setup organization so Authorization middleware works
	db := datastore.New(ctx)

	org := organization.New(db)
	org.Name = "org"
	org.MustCreate()

	nsDb := datastore.New(org.Namespaced(ctx))
	ap := app.New(nsDb)
	ap.MustCreate()

	t1, err := ap.NewApiKey("tok1", token.Claims{
		Permissions: bit.Field(Perm1),
	})
	Expect(err).NotTo(HaveOccurred())
	tok1 = t1.String

	t2, err := ap.NewApiKey("tok2", token.Claims{
		Permissions: bit.Field(Perm2 | Perm3),
	})
	Expect(err).NotTo(HaveOccurred())
	tok2 = t2.String
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	inst.Close()
})

var _ = Describe("New", func() {
	It("Should create a new Rest object with CRUD routes", func() {
		client := ginclient.New(ctx)

		// Create routes for Model
		r := rest.New(user.User{})
		r.Permissions = rest.Permissions{
			"get":  []bit.Mask{Perm1, Perm2 | Perm3},
			"list": []bit.Mask{Perm1, Perm2 | Perm3},
		}
		r.Route(client.Router, middleware.TokenRequired())

		// Should not be authorized
		w := client.Get("/user")
		Expect(w.Code).To(Equal(401))

		// Set authorization header for subsequent requests
		client.Setup(func(r *http.Request) {
			r.Header.Set("Authorization", tok1)
		})

		// Get should work ok
		w = client.Get("/user")
		Expect(w.Code).To(Equal(200))

		// Should 404
		w = client.Get("/user2")
		Expect(w.Code).To(Equal(404))

		// Should work with more complex token
		client.Setup(func(r *http.Request) {
			r.Header.Set("Authorization", tok2)
		})

		// Get should work ok
		w = client.Get("/user")
		Expect(w.Code).To(Equal(200))

		// Should 404
		w = client.Get("/user2")
		Expect(w.Code).To(Equal(404))
	})
})
