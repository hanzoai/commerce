package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/gincontext"
	"crowdstart.io/util/test/ae"

	. "crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("auth", t)
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
				u, err := auth.NewUser(c, &regForm)
				Expect(u.Id).ToNot(Equal(""))
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

				_, err := auth.NewUser(c, &regForm)
				Expect(err).To(HaveOccurred())
			})
		})
	})
}
