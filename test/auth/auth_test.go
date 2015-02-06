package test

import (
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"github.com/zeekay/aetest"
)

func TestDatastore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "auth")
}

const kind = "user"

var (
	ctx aetest.Context
	db  *datastore.Datastore
	c   *gin.Context
)

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
	Context("Registering with unique email", func() {
		It("should not error", func() {
			regForm := auth.RegistrationForm{
				User:     models.User{Email: "a@example.com"},
				Password: "hunter2",
			}
			err := auth.NewUser(c, &regForm)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Query api get", func() {
		It("should not error", func() {
			regForm := auth.RegistrationForm{
				User:     models.User{Email: "b@example.com"},
				Password: "hunter2",
			}
			auth.NewUser(c, &regForm)

			keys, err := db.Query(kind).
				Filter("Email =", regForm.User.Email).
				KeysOnly().Limit(1).GetAll(ctx, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(keys).To(HaveLen(1))
		})
	})

	Context("Re-registering", func() {
		It("should error", func() {
			regForm := auth.RegistrationForm{
				User:     models.User{Email: "b@example.com"},
				Password: "hunter2",
			}
			auth.NewUser(c, &regForm)

			err := auth.NewUser(c, &regForm)
			Expect(err).To(HaveOccurred())
		})
	})
})
