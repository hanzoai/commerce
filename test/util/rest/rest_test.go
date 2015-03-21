package test

import (
	"net/http"
	"testing"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/test/ae"
	"crowdstart.io/util/test/ginclient"

	. "crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("util/rest", t)
}

var (
	ctx         ae.Context
	accessToken string
)

// Setup appengine context
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()

	// Setup organization so Authorization middleware works
	db := datastore.New(ctx)
	org := organization.New(db)
	accessToken = org.AddToken("admin", permission.Admin)
	err := org.Put()
	Expect(err).NotTo(HaveOccurred())
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

type Model struct {
	Name string
}

func (m Model) Kind() string {
	return "test-model"
}

var _ = Describe("New", func() {
	It("Should create a new Rest object with CRUD routes", func() {
		client := ginclient.New(ctx)

		// Create routes for Model
		rest := rest.New(Model{})
		rest.Route(client.Router)

		// Should not be authorized
		w := client.Get("/test-model")
		Expect(w.Code).To(Equal(401))

		// Set authorization header for subsequent requests
		client.Setup(func(r *http.Request) {
			r.Header.Set("Authorization", accessToken)
		})

		// Get should work ok
		w = client.Get("/test-model")
		Expect(w.Code).To(Equal(200))

		// Should 404
		w = client.Get("/test-model2")
		Expect(w.Code).To(Equal(404))
	})
})
