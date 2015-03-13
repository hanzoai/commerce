package test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/test/ae"
	"crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	ginkgo.Setup("models", t)
}

var (
	ctx ae.Context
	db  *datastore.Datastore
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

type User struct {
	mixin.Model
	Name string
}

func (u *User) Kind() string {
	return "user"
}

func NewUser(db *datastore.Datastore) *User {
	u := new(User)
	u.Model = mixin.Model{Db: db, Entity: u}
	return u
}

var _ = Describe("models/mixin", func() {
	Context("Model.Put", func() {
		It("should save entity to datastore", func() {
			// Create a new user and store using Model mixin
			user := NewUser(db)
			user.Name = "Justin"
			user.Put()

			// Manually retrieve to ensure it was saved properly
			user2 := new(User)
			db.Get(user.Key(), user2)
			Expect(user2.Name).To(Equal(user.Name))
		})
	})

	Context("Model.Get", func() {
		It("should retrieve entity from datastore", func() {
			// Manually create a new user and store in datastore
			user := new(User)
			user.Name = "Dustin"
			key, err := db.Put("user", user)
			Expect(err).NotTo(HaveOccurred())

			// Retrieve user from datastore using Model mixin
			user2 := NewUser(db)
			user2.Get(key)
			Expect(user2.Name).To(Equal(user.Name))
		})
	})
})
