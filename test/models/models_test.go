package test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/zeekay/aetest"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/log"
)

func TestDatastore(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	RegisterFailHandler(Fail)
	RunSpecs(t, "models")
}

var (
	ctx aetest.Context
	db  *datastore.Datastore
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	var err error
	ctx, err = aetest.NewContext(&aetest.Options{StronglyConsistentDatastore: true})
	Expect(err).NotTo(HaveOccurred())
	db = datastore.New(ctx)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	err := ctx.Close()
	Expect(err).NotTo(HaveOccurred())
})

type User struct {
	mixin.Model `datastore:"-"`
	Name        string
}

func (u *User) Kind() string {
	return "user"
}

func NewUser(db *datastore.Datastore) *User {
	user := new(User)
	user.Model = mixin.NewModel(db, user)
	return user
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
