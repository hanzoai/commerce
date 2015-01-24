package test

import (
	"testing"

	gaed "appengine/datastore"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"github.com/zeekay/aetest"
)

const kind = "user"

var (
	ctx aetest.Context
	db  *datastore.Datastore
	c   *gin.Context
)

func TestDatastore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Datastore test suite")
}

var _ = BeforeSuite(func() {
	var err error
	ctx, err = aetest.NewContext(&aetest.Options{StronglyConsistentDatastore: true})
	Expect(err).ToNot(HaveOccurred())
	c = &gin.Context{}
	c.Set("appengine", ctx)
})

var _ = AfterSuite(func() {
	err := ctx.Close()
	Expect(err).ToNot(HaveOccurred())
})

var _ = Describe("NewUser", func() {
	var regForm auth.RegistrationForm
	Context("Registering with unique email", func() {
		It("should not error", func() {
			regForm = auth.RegistrationForm{
				User:     models.User{Email: "e@example.com"},
				Password: "hunter2",
			}
			err := auth.NewUser(c, &regForm)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Query api get", func() {
		var keys []*gaed.Key
		It("should not error", func() {
			var err error
			keys, err = db.Query(kind).
				Filter("Email =", regForm.User.Email).
				KeysOnly().Limit(1).GetAll(ctx, nil)
			Expect(err).ToNot(HaveOccurred())
		})
		It("should return one key", func() {
			Expect(keys).To(HaveLen(1))
		})
	})

	Context("Re-registering", func() {
		It("should error", func() {
			err := auth.NewUser(c, &regForm)
			Expect(err).To(HaveOccurred())
		})
	})
})
