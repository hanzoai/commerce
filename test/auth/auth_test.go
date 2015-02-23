package test

import (
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/gincontext"
	"crowdstart.io/util/test/ae"
	"crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	ginkgo.Setup("auth", t)
}

const kind = "user"

var (
	c   *gin.Context
	ctx ae.Context
	db  *datastore.Datastore
)

func init() {
	BeforeSuite(func() {
		ctx = ae.NewContext()
		c = gincontext.New(ctx)
		db = datastore.New(ctx)
	})
	AfterSuite(func() {
		ctx.Close()
	})

	Describe("NewUser", func() {
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
}
