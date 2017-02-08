package test

import (
	"net/http"
	"testing"

	"golang.org/x/net/context"
	"google.golang.org/appengine"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/app"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/ginclient"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("middleware/accesstoken", t)
}

const kind = "user"

var (
	ctx  context.Context
	inst ae.Instance
	db   *datastore.Datastore
)

// Setup appengine context, gin context, and datastore before tests
var _ = BeforeSuite(func() {
	ctx, inst, _ = ae.NewContext()
	db = datastore.New(ctx)
})

var _ = AfterSuite(func() {
	inst.Close()
})

type Stub struct {
	Foo string
}

var _ = Describe("middleware/accesstoken", func() {
	Context("siteToken.RequiresOrgToken", func() {
		It("should namespace based on Organization.Name", func() {
			u := user.New(db)
			u.MustPut()

			// create an org
			o := organization.New(db)
			o.Name = "Org"
			o.Owners = []string{u.Id()}

			// insert into db
			o.MustCreate()

			id := o.Name

			nsDb := datastore.New(o.Namespaced(db.Context))
			ap := app.New(nsDb)
			ap.GetById(organization.DefaultAppName)

			// get siteToken
			testTok, _, err := ap.GetApiKeyByName(app.TestPublishedKey)
			Expect(err).ToNot(HaveOccurred())
			siteToken := testTok.String

			// Make request using access token
			client := ginclient.Middleware(ctx, middleware.TokenRequired())
			client.Setup(func(r *http.Request) {
				r.Header.Set("Authorization", siteToken)
			})
			client.Get("/")

			// middleware generates namespaced appengine context
			o2 := middleware.GetOrganization(client.Context)
			ap2 := middleware.GetApp(client.Context)

			ctx2 := o2.Namespaced(ctx)
			Expect(ctx2).ToNot(Equal(ctx))
			Expect(o2).ToNot(Equal(nil))
			Expect(ap2).ToNot(Equal(nil))

			// make db from namespaced context
			db2 := datastore.New(ctx2)

			stub2 := Stub{Foo: "1"}
			key2, err := db2.Put("namespaced-things", &stub2)
			Expect(err).ToNot(HaveOccurred())

			// make another namespace context different from returned
			ctx3, err := appengine.Namespace(ctx, "empty-namespace")
			Expect(err).ToNot(HaveOccurred())

			// make db from different namespace context
			db3 := datastore.New(ctx3)

			// shouldn't be able to get namespaced key
			stub3 := Stub{}
			key3 := db3.NewKey(key2.Kind(), key2.StringID(), key2.IntID(), nil)
			err = db3.Get(key3, &stub3)
			Expect(err).To(HaveOccurred())
			Expect(stub3.Foo).ToNot(Equal(stub2.Foo))

			// make another namespace context same as returned
			ctx4, err := appengine.Namespace(ctx, id)
			Expect(err).ToNot(HaveOccurred())

			// make db from same namespace context
			db4 := datastore.New(ctx4)

			stub4 := Stub{}
			key4 := db2.NewKey(key2.Kind(), key2.StringID(), key2.IntID(), nil)
			err = db2.Get(key4, &stub4)
			Expect(err).ToNot(HaveOccurred())
			Expect(stub4.Foo).To(Equal(stub2.Foo))
			err = db4.Get(key4, &stub4)
		})
	})
})
