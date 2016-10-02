package test

import (
	"net/http"
	"testing"

	"appengine"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"
	"crowdstart.com/util/test/ae"
	"crowdstart.com/util/test/ginclient"

	. "crowdstart.com/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("middleware/accesstoken", t)
}

const kind = "user"

var (
	ctx ae.Context
	db  *datastore.Datastore
)

// Setup appengine context, gin context, and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)
})

var _ = AfterSuite(func() {
	ctx.Close()
})

type Stub struct {
	Foo string
}

var _ = Describe("middleware/accesstoken", func() {
	Context("accessToken.RequiresOrgToken", func() {
		It("should namespace based on Organization.Name", func() {
			u := user.New(db)
			err := u.Put()
			Expect(err).NotTo(HaveOccurred())

			// create an org
			o := organization.New(db)
			o.Name = "Justin"
			o.SecretKey = []byte("AAA")
			o.Owners = []string{u.Id()}

			// insert into db
			err = o.Put()
			Expect(err).NotTo(HaveOccurred())

			id := o.Name

			// generate accessToken
			accessToken := o.AddToken("some-token", 0)

			// Update organization, so middleware can find it
			err = o.Put()
			Expect(err).NotTo(HaveOccurred())

			// Make request using access token
			client := ginclient.New(ctx)
			client.Use(middleware.TokenRequired())
			client.Defaults(func(r *http.Request) {
				r.Header.Set("Authorization", accessToken)
			})
			client.Get("/", nil)

			// middleware generates namespaced appengine context
			org := middleware.GetOrganization(client.Context)

			ctx2 := org.Namespaced(ctx)
			Expect(ctx2).ToNot(Equal(ctx))
			Expect(org).ToNot(Equal(nil))

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
			key4 := db4.NewKey(key2.Kind(), key2.StringID(), key2.IntID(), nil)
			err = db4.Get(key4, &stub4)
			Expect(err).ToNot(HaveOccurred())
			Expect(stub4.Foo).To(Equal(stub2.Foo))
		})
	})
})
