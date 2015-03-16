package test

import (
	"net/http"
	"testing"
	"time"

	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/user"
	"crowdstart.io/util/gincontext"
	"crowdstart.io/util/test/ae"

	. "crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("middleware/accesstoken", t)
}

const kind = "user"

var (
	c   *gin.Context
	ctx ae.Context
	db  *datastore.Datastore
)

// Setup appengine context, gin context, and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	c = gincontext.New(ctx)
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
		It("should namespace based on org id", func() {
			// add a dummy request
			req, err := http.NewRequest("GET", "", nil)
			Expect(err).NotTo(HaveOccurred())

			c.Request = req

			defer c.Set("appengine", ctx)

			u := user.New(db)
			u.Put()

			// create an org
			o := organization.New(db)
			o.Name = "Justin"
			o.IssuedAt = time.Now()
			o.SecretKey = []byte("AAA")
			o.Owners = []string{u.Id()}

			// insert into db
			o.Put()

			id := o.Id()

			// generate accessToken
			tokenStr, err := o.GenerateAccessToken(u)
			Expect(err).NotTo(HaveOccurred())

			// get the middleware func`
			gFunc := middleware.TokenRequired()

			// set the access token on the request header
			c.Request.Header.Set("Authorization", tokenStr)

			// middleware generates namespaced appengine context
			gFunc(c)
			org := middleware.GetOrg(c)
			ctx2, err := org.Namespace(ctx)
			Expect(err).NotTo(HaveOccurred())
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
